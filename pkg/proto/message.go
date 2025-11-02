package proto

// ClientToServerMessage represents a message from the client to the server.
type ClientToServerMessage struct {
	Type     string `json:"type" validate:"required"`
	Position []int  `json:"position" validate:"required,len=2,dive,min=0,max=2"`
}

// ServerToClientMessage represents a message from the server to the client.
type ServerToClientMessage struct {
	Type   string     `json:"type"`
	Board  [][]string `json:"board"`
	Next   string     `json:"next"`
	Winner string     `json:"winner,omitempty"`
}

// PlayerAssignmentMessage informs a player of their assigned mark.
type PlayerAssignmentMessage struct {
	Type string `json:"type"`
	Mark string `json:"mark"`
}
