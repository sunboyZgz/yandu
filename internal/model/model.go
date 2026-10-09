package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const Version = "0.1.0"
const ManagementAddr = "127.0.0.1:18765"
const ResourceAddr = "127.0.0.1:18766"
const GuardAddr = "127.0.0.1:18767"
const CaddyAdmin = "127.0.0.1:18769"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string       { return e.Code + ": " + e.Message }
func Err(code, message string) error { return &Error{code, message} }
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

type Site struct {
	ID                string   `json:"id"`
	Domain            string   `json:"domain"`
	AllowedPrefixes   []string `json:"allowedPrefixes"`
	ProtectedPrefixes []string `json:"protectedPrefixes"`
}
type Profile struct {
	SchemaVersion  int    `json:"schemaVersion"`
	DeviceID       string `json:"deviceId"`
	ServerAddr     string `json:"serverAddr"`
	ServerPort     int    `json:"serverPort"`
	TLSName        string `json:"tlsName"`
	TunnelCA       string `json:"tunnelCA"`
	TunnelToken    string `json:"tunnelToken"`
	SSHAddr        string `json:"sshAddr"`
	SSHUser        string `json:"sshUser"`
	SSHFingerprint string `json:"sshFingerprint"`
	SSHPrivateKey  string `json:"sshPrivateKey"`
	Sites          []Site `json:"sites"`
}
type Project struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"displayName"`
	SiteID           string   `json:"siteId"`
	ProjectBase      string   `json:"projectBase"`
	RootPath         string   `json:"rootPath"`
	ResourcePrefixes []string `json:"resourcePrefixes"`
	Publication      string   `json:"publication"`
	AccessPolicy     string   `json:"accessPolicy"`
	CachePolicy      string   `json:"cachePolicy"`
	ContentPolicy    string   `json:"contentPolicy"`
	Revision         int64    `json:"revision"`
	Status           string   `json:"status"`
	LastError        string   `json:"lastError,omitempty"`
	Username         string   `json:"username,omitempty"`
	PasswordHash     string   `json:"passwordHash,omitempty"`
	Generation       string   `json:"generation,omitempty"`
	ProbeToken       string   `json:"probeToken,omitempty"`
	AppliedRevision  int64    `json:"appliedRevision"`
}

func (p Project) Public() Project {
	p.PasswordHash = ""
	p.ProbeToken = ""
	p.Generation = ""
	return p
}

type Route struct {
	ID           string `json:"routeId"`
	ProjectID    string `json:"projectId"`
	Prefix       string `json:"prefix"`
	State        string `json:"state"`
	AccessPolicy string `json:"accessPolicy"`
}

func RouteID(id, prefix string) string { return fmt.Sprintf("%s-%s", id, Hex([]byte(prefix))[:12]) }

type Manifest struct {
	SchemaVersion         int      `json:"schemaVersion"`
	OperationID           string   `json:"operationId"`
	SiteID                string   `json:"siteId"`
	ExpectedCloudRevision int64    `json:"expectedCloudRevision"`
	SourceRevision        int64    `json:"sourceRevision"`
	Routes                []Route  `json:"routes"`
	Release               []string `json:"release,omitempty"`
}
type Receipt struct {
	OperationID string `json:"operationId"`
	SiteID      string `json:"siteId"`
	Revision    int64  `json:"revision"`
	Generation  string `json:"generation"`
	Verified    bool   `json:"verified"`
	Error       *Error `json:"error,omitempty"`
}
type Operation struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Action    string    `json:"action"`
	Stage     string    `json:"stage"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Snapshot struct {
	Revision        int64               `json:"revision"`
	AppliedRevision int64               `json:"appliedRevision"`
	Projects        []Project           `json:"projects"`
	Effective       []Project           `json:"effective"`
	Roots           []string            `json:"roots"`
	Profile         *Profile            `json:"profile,omitempty"`
	Operations      []Operation         `json:"operations"`
	CloudRevisions  map[string]int64    `json:"cloudRevisions"`
	CloudDigests    map[string]string   `json:"cloudDigests"`
	Reserved        map[string][]Route  `json:"reserved"`
	Pending         map[string]Manifest `json:"pending"`
}

func Empty() Snapshot {
	return Snapshot{Projects: []Project{}, Effective: []Project{}, Roots: []string{}, Operations: []Operation{}, CloudRevisions: map[string]int64{}, CloudDigests: map[string]string{}, Pending: map[string]Manifest{}, Reserved: map[string][]Route{}}
}
func (s Snapshot) Site(id string) (Site, bool) {
	if s.Profile != nil {
		for _, v := range s.Profile.Sites {
			if v.ID == id {
				return v, true
			}
		}
	}
	return Site{}, false
}
