package core

import (
	"context"
	"crypto/x509"
	"embed"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/zveinn/crypt"
)

var (
	PRODUCTION  = true
	APP_VERSION = "1.0.4"
	API_VERSION = 1

	DefaultTunnelName    = "tunnels"
	DefaultTunnelNameMin = "tunnels-min"
	CertPool             *x509.CertPool

	DNSLock         = sync.Mutex{}
	DNSBlockedList  = make(map[string]*DNSStats)
	DNSResolvedList = make(map[string]*DNSStats)
	DNSCache        = make(map[string]*DNSReply)
	DNSCacheLock    = sync.Mutex{}
	UsePrimaryDNS   = true
)

type DNSStats struct {
	Count     int
	Tag       string
	LastSeen  time.Time
	FirstSeen time.Time
	Answers   []string
}

type ConnectionRequest struct {
	DeviceKey string `json:"DeviceKey"`

	DeviceToken string `json:"DeviceToken"`
	UserID      string `json:"UserID"`

	Tag        string          `json:"Tag"`
	ServerID   string          `json:"ServerID"`
	ServerIP   string          `json:"ServerIP"`
	ServerPort string          `json:"ServerPort"`
	EncType    crypt.EncType   `json:"EncType"`
	CurveType  crypt.CurveType `json:"CurveType"`
}

type RemoteConnectionRequest struct {
	// CLI/MIN
	DeviceKey string `json:"DeviceKey"`

	// GUI
	DeviceToken string `json:"DeviceToken"`
	UserID      string `json:"UserID"`

	// General
	EncType   crypt.EncType   `json:"EncType"`
	CurveType crypt.CurveType `json:"CurveType"`
	SeverID   string          `json:"ServerID"`
	Serial    string          `json:"Serial"`

	// These are added by the golang client
	Version int       `json:"Version"`
	Created time.Time `json:"Created"`

	RequestingPorts bool   `json:"RequestingPorts"`
	DHCPToken       string `json:"DHCPToken"`
	Hostname        string `json:"Hostname"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}

type SignedConnectRequest struct {
	Signature []byte
	Payload   []byte
}

type ConnectRequestResponse struct {
	Index             int `json:"Index"`
	AvailableMbps     int `json:"AvailableMbps"`
	AvailableUserMbps int `json:"AvailableUserMbps"`

	InternetAccess     bool `json:"InternetAccess,required"`
	LocalNetworkAccess bool `json:"LocalNetworkAccess"`

	DataPort    string `json:"DataPort"`
	InterfaceIP string `json:"InterfaceIP"`

	// Normal VPN
	StartPort uint16 `json:"StartPort"`
	EndPort   uint16 `json:"EndPort"`

	DNS                []*ServerDNS     `json:"DNS"`
	Networks           []*ServerNetwork `json:"Networks"`
	DNSServers         []string         `json:"DNSServers"`
	DNSAllowCustomOnly bool             `json:"DNSAllowCustomOnly"`

	// VPL Mapping
	DHCP       *DHCPRecord    `json:"DHCP"`
	VPLNetwork *ServerNetwork `json:"VPLNetwork"`
}

type DHCPRecord struct {
	IP       [4]byte
	Token    string
	Hostname string
}

var (
	DIST_EMBED embed.FS
	DLL_EMBED  embed.FS
)

func initializeMinimalGlobalVariables() {
	C = new(Config)
	C.DebugLogging = true
	C.InfoLogging = true
	C.ConsoleLogOnly = true
}

func initializeGlobalVariables() {
	C = new(Config)
	C.DebugLogging = true
	C.InfoLogging = true

	GLOBAL_STATE.DNSBlocksMap = make(map[string]*DNSStats)
	GLOBAL_STATE.DNSResolvesMap = make(map[string]*DNSStats)
}

var (
	AppStartTime  = time.Now()
	C             = new(Config)
	GLOBAL_STATE  = new(State)
	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	DEFAULT_TUNNEL      *TunnelInterface
	DEFAULT_DNS_SERVERS []string
	DNSClient           = new(dns.Client)

	// DEFAULT CONNECTION
	DEFAULT_CONNECTION *TunnelMETA

	// IS POPUI GUI
	POPUI  bool
	uiChan = make(chan struct{}, 1)

	MINIMAL bool

	// Device Flags
	CLIDeviceKey          string
	CLIDNS                string
	CLIHost               string
	CLIHostname           string
	CLIPort               string
	CLIServerID           string
	CLIDisableBlockLists  bool
	CLIDisableVPLFirewall bool

	// Base Path Overwrite
	BASE_PATH string

	// HTTP
	API_SERVER http.Server
	API_PORT   string

	// INTERFACE RELATED
	DEFAULT_GATEWAY         net.IP
	DEFAULT_INTERFACE       net.IP
	DEFAULT_INTERFACE_ID    int
	DEFAULT_INTERFACE_NAME  string
	ROUTER_PROBE_TIMEOUT_MS = 60000
	LAST_ROUTER_PROBE       = time.Now().AddDate(0, 0, -1)

	CURRENT_UBBS           = 0
	CURRENT_DBBS           = 0
	EGRESS_PACKETS  uint64 = 0
	INGRESS_PACKETS uint64 = 0

	LogQueue          = make(chan string, 1000)
	APILogQueue       = make(chan string, 1000)
	TAG_ERROR         = "ERROR"
	TAG_GENERAL       = "GENERAL"
	LogFile           *os.File
	TraceFile         *os.File
	UDPDNSServer      *dns.Server
	BLOCK_DNS_QUERIES = false
)

type DNSReply struct {
	// M       *dns.Msg
	A       []dns.RR
	Expires time.Time
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

// var LastRouterPing = time.Now()
var LastConnectionAttemp = time.Now()

var (
	BUFFER_ERROR             bool
	IGNORE_NEXT_BUFFER_ERROR bool
)

var DNSWhitelist = make(map[string]bool)

type IP struct {
	LOCAL  map[uint16]*RemotePort
	REMOTE map[uint16]*RemotePort
}

type RemotePort struct {
	Local        uint16
	Original     uint16
	Mapped       uint16
	LastActivity time.Time
}

type LogItem struct {
	Type string
	Line string
}

type LogoutForm struct {
	Email       string
	DeviceToken string
}

type State struct {
	IsAdmin bool    `json:"IsAdmin"`
	C       *Config `json:"C"`
	User    User

	UMbps           int    `json:"UMbps"`
	DMbps           int    `json:"DMbps"`
	UMbpsString     string `json:"UMbpsString"`
	DMbpsString     string `json:"DMbpsString"`
	IngressPackets  uint64 `json:"IngressPackets"`
	EgressPackets   uint64 `json:"EgressPackets"`
	ConnectionStats []TunnelSTATS

	LastNodeUpdate         time.Time
	SecondsUntilNodeUpdate int

	AvailableCountries []string `json:"AvailableCountries"`

	// FILE PATHS
	BlockListPath string `json:"BlockListPath"`
	TraceFileName string `json:"TraceFileName"`
	TracePath     string `json:"TracePath"`
	LogFileName   string `json:"LogFileName"`
	LogPath       string `json:"LogPath"`
	ConfigPath    string `json:"ConfigPath"`
	BasePath      string `json:"BasePath"`
	Version       string `json:"Version"`

	ActiveConnections []*TunnelMETA `json:"ActiveConnections"`

	// DNS stats
	DNSBlocksMap   map[string]*DNSStats `json:"DNSBlocks"`
	DNSResolvesMap map[string]*DNSStats `json:"DNSResolves"`
}

type List struct {
	FullPath string
	Tag      string
	Enabled  bool
	Domains  string
}

type DisconnectForm struct {
	GUID string `json:"GUID"`
}

type CONFIG_FORM struct {
	DNS1                      string   `json:"DNS1"`
	DNS2                      string   `json:"DNS2"`
	ManualRouter              bool     `json:"ManualRouter"`
	Region                    string   `json:"Region"`
	Version                   string   `json:"Version"`
	RouterFilePath            string   `json:"RouterFilePath"`
	DebugLogging              bool     `json:"DebugLogging"`
	AutoReconnect             bool     `json:"AutoReconnect"`
	KillSwitch                bool     `json:"KillSwitch"`
	DisableIPv6OnConnect      bool     `json:"DisableIPv6OnConnect"`
	CloseConnectionsOnConnect bool     `json:"CloseConnectionsOnConnect"`
	CustomDNS                 bool     `json:"CustomDNS"`
	EnabledBlockLists         []string `json:"EnabledBlockLists"`
	LogBlockedDomains         bool     `json:"LogBlockedDomains"`
}

var (
	TunList [1000]*Tunnel
	ConLock = sync.Mutex{}
	IFList  [1000]*TunnelInterface
	IFLock  = sync.Mutex{}
)

type ConnectionOverwrite struct {
	ServerID string `json:"ServerID"`
	Network  string `json:"Network" bson:"Network"`
	Nat      string `json:"Nat" bson:"Nat"`
}

type Route struct {
	Address string
	Metric  string
}

type ServerDNS struct {
	Domain   string   `json:"Domain"`
	Wildcard bool     `json:"Wildcard" bson:"Wildcard"`
	IP       []string `json:"IP" bson:"IP"`
	TXT      []string `json:"TXT" bson:"TXT"`
	CNAME    string   `json:"CNAME" bson:"CNAME"`
}
type ServerNetwork struct {
	Tag     string   `json:"Tag" bson:"Tag"`
	Network string   `json:"Network" bson:"Network"`
	Nat     string   `json:"Nat" bson:"Nat"`
	Routes  []*Route `json:"Routes" bson:"Routes"`

	// Post Init
	NatIPNet *net.IPNet `json:"-"`
	NetIPNet *net.IPNet `json:"-"`
}

type ActiveConnectionMeta struct {
	Country        string
	RouterIndex    int
	NodeID         string
	Tag            string
	LocalInterface string
	IPv4Address    string
	IPv6Address    string
	StartPort      uint16
	EndPort        uint16
}

type TunnelMETA struct {
	Private          bool
	PrivateIP        string
	PrivatePort      string
	PrivateCert      string
	PrivateCertBytes []byte `json:"-"`
	DNSDiscovery     string

	OrgID     string
	DeviceKey string
	DHCPToken string
	ServerID  string
	Hostname  string

	WindowsGUID string

	// controlled by user only
	DNSBlocking     bool
	LocalhostNat    bool
	AutoReconnect   bool
	AutoConnect     bool
	Persistent      bool
	PreventIPv6     bool
	RequestVPNPorts bool

	EncryptionType crypt.EncType
	CurveType      crypt.CurveType

	// EXPERIMENTAL
	CloseConnectionsOnConnect bool

	// Is delivered from company but can be overwirtten by user
	TxQueueLen int32
	MTU        int32
	IFName     string

	// IS controller by ORG if user is part of one
	// ID                  primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Tag         string
	IPv4Address string
	IPv6Address string
	NetMask     string

	// VPL Firewall
	AllowedHosts    []string
	DisableFirewall bool

	// Port blocking
	// BlockedPorts       []int
	// ParsedBlockedPorts [][]byte

	// This overwrites or adds to settings
	// that are applied to the Node
	EnableDefaultRoute bool
	DNSServers         []string
	DNS                []*ServerDNS
	Networks           []*ServerNetwork
}

type AllowedHost struct {
	Host    string
	Expires time.Time
}

// func (VC *VPNConnection) Initialize() {
// 	if len(VC.Node.DNSServers) > 0 {
// 		VC.DNS1IP = net.ParseIP(VC.Node.DNSServers[0]).To4()
// 		VC.DNS1Bytes = [4]byte(VC.DNS1IP)
// 	} else {
// 		VC.DNS1IP = net.ParseIP(C.DNS1Default).To4()
// 		VC.DNS1Bytes = [4]byte(VC.DNS1IP)
// 	}
// }

type TunnelSTATS struct {
	// Stats
	StatsTag      string
	EgressBytes   int
	EgressString  string
	IngressBytes  int
	IngressString string

	// Port range on server
	StartPort uint16
	EndPort   uint16

	// Security stuff
	Nonce1 uint64
	Nonce2 uint64

	// FROM NODE
	CPU                 byte
	DISK                byte
	MEM                 byte
	ServerToClientMicro int64
	PingTime            time.Time

	DHCP       *DHCPRecord
	VPLNetwork *ServerNetwork
}

type FirewallRequest struct {
	DHCPToken       string
	IP              string
	Hosts           []string
	DisableFirewall bool
}

type Tunnel struct {
	Meta *TunnelMETA
	TunnelSTATS
	CRR      *ConnectRequestResponse
	ClientCR ConnectionRequest
	Con      net.Conn

	// TUN/TAP
	Index        []byte
	Nonce2Bytes  []byte
	Interface    *TunnelInterface
	AddressNetIP net.IP
	Routes       []string

	// ??????
	// CRR       *ConnectRequestResponse
	StartPort uint16
	EndPort   uint16
	EH        *crypt.SocketWrapper

	// STATES
	Connected              bool
	UserRWLoopAbnormalExit bool
	Connecting             bool
	Exiting                bool

	// VPN NODE
	LOCAL_IF_IP [4]byte

	PingBuffer [8]byte

	// DNS1Bytes     [4]byte `json:"-"`
	// DNS1IP        net.IP  `json:"-"`
	PrevDNS       net.IP
	DNSBytes      [4]byte
	DNSIP         net.IP
	DNSEgressLock sync.Mutex

	// NEW PORT MAPPING
	TCP_M  []VPNPort
	UDP_M  []VPNPort
	TCP_EM map[[10]byte]*Mapping
	UDP_EM map[[10]byte]*Mapping
	EP_MP  *Mapping
	IP_MP  *Mapping
	EP_SYN byte

	// VPL
	VPL_IP    [4]byte
	VPL_E_MAP map[[4]byte]struct{} `json:"-"`
	VPL_I_MAP map[[4]byte]struct{} `json:"-"`

	//  NAT
	NAT_CACHE         map[[4]byte][4]byte `json:"-"`
	REVERSE_NAT_CACHE map[[4]byte][4]byte `json:"-"`

	// BufferError bool

	//  PACKET MANIPULATION
	EP_Version  byte
	EP_Protocol byte

	EP_DstIP [4]byte

	EP_IPv4HeaderLength byte
	EP_IPv4Header       []byte
	EP_TPHeader         []byte

	EP_SrcPort [2]byte
	EP_DstPort [2]byte

	EP_NAT_IP [4]byte
	EP_NAT_OK bool

	EP_DNS_Response         []byte
	EP_DNS_Local            bool
	EP_DNS_Drop             bool
	EP_DNS_Forward          bool
	EP_DNS_Port_Placeholder [2]byte
	EP_DNS_Packet           []byte

	// This IP gets over-written on connect
	EP_VPNSrcIP [4]byte

	// EP_NEW_RST  byte
	PREV_DNS_IP [4]byte
	IS_UNIX     bool

	IP_Version  byte
	IP_Protocol byte

	IP_DstIP [4]byte
	IP_SrcIP [4]byte

	IP_IPv4HeaderLength byte
	IP_IPv4Header       []byte
	IP_TPHeader         []byte

	IP_SrcPort [2]byte
	IP_DstPort [2]byte

	IP_NAT_IP [4]byte
	IP_NAT_OK bool
}

type Config struct {
	Connections []*TunnelMETA

	DarkMode bool

	// Security settings
	IsolationMode bool

	// API Setting
	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType

	// Optional Debugging Settings
	LogBlockedDomains bool
	LogAllDomains     bool
	DebugLogging      bool
	DeepDebugLoggin   bool
	ConsoleLogging    bool
	InfoLogging       bool
	ErrorLogging      bool
	ConsoleLogOnly    bool
	ConnectionTracer  bool

	// DNS Settings
	DNS1Default         string
	DNS2Default         string
	DNSOverHTTPS        bool
	DNSstats            bool
	DNSServerIP         string
	DNSServerPort       string
	DomainWhitelist     string
	EnabledBlockLists   []string
	AvailableBlockLists []*BlockList

	DNSRecords []*ServerDNS
}

var (
	DNSBlockList = make(map[string]string)
	DNSBlockLock = sync.Mutex{}
)

type LOADING_LOGS_RESPONSE struct {
	Lines [100]string
}
type GENERAL_LOGS_RESPONSE struct {
	Lines []string
}
type GeneralLogResponse struct {
	Content  []string
	Time     []string
	Function []string
	Color    []string
}

type DEBUG_OUT struct {
	Lines []string
	File  string
}

type FORWARD_REQUEST struct {
	Path    string
	Method  string
	Timeout int
	Authed  bool
	// Data     []byte
	JSONData interface{}
}

type TWO_FACTOR_CONFIRM struct {
	Email  string
	Code   string
	Digits string
}

type QR_CODE struct {
	Value string
	// Recovery string
}

// Device token struct need for the login respons from user scruct
type DEVICE_TOKEN struct {
	DT      string    `bson:"DT"`
	N       string    `bson:"N"`
	Created time.Time `bson:"C"`
}

// use struct you get from the login request
type User struct {
	ID                    string          `json:"_id,omitempty"`
	APIKey                string          `json:"APIKey"`
	Email                 string          `json:"Email"`
	DeviceToken           *DEVICE_TOKEN   `json:",omitempty"`
	Tokens                []*DEVICE_TOKEN `json:"Tokens"`
	OrgID                 string          `json:"OrgID" `
	Key                   *LicenseKey     `json:"Key"`
	Trial                 bool            `json:"Trial"`
	Disabled              bool            `json:"Disabled"`
	TwoFactorEnabled      bool            `json:"TwoFactorEnabled"`
	Updated               time.Time       `json:"Updated"`
	SubExpiration         time.Time       `json:"SubExpiration"`
	AdditionalInformation string          `json:"AdditionalInformation,omitempty"`
}

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}

type BlockList struct {
	Tag         string
	FullPath    string
	DiskPath    string
	Enabled     bool
	Count       int
	LastRefresh time.Time
}
