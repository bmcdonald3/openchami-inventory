package connection

import "github.com/openchami/inventory/pkg/resources"

// Connection represents a physical or logical link between two device ports.
type Connection struct {
	resources.Resource
	Spec   ConnectionSpec   `json:"spec"`
	Status ConnectionStatus `json:"status,omitempty"`
}

// ConnectionSpec defines the desired state of a Connection.
type ConnectionSpec struct {
	// A simple, human-friendly, auto-incrementing integer ID.
	NumericID int `json:"numericId,omitempty"`

	// The kind of connection (e.g., "Ethernet", "Power").
	ConnectionType string `json:"connectionType"`

	// The ID of the physical cable device, if tracked.
	MediumID string `json:"mediumId,omitempty"`

	// The first connection point.
	EndpointA Endpoint `json:"endpointA"`

	// The second connection point.
	EndpointB Endpoint `json:"endpointB"`
}

// Endpoint defines one side of a connection, linking a device and a port.
type Endpoint struct {
	DeviceID string `json:"deviceId"`
	PortName string `json:"portName"`
}

// ConnectionStatus defines the observed state of a Connection.
type ConnectionStatus struct {
	// Represents the latest available observations of a resource's state.
	Conditions []resources.Condition `json:"conditions,omitempty"`
}
