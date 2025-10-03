package device

import "github.com/openchami/inventory/pkg/resources"

// Device represents a single piece of hardware in the system.
type Device struct {
	resources.Resource
	Spec   DeviceSpec   `json:"spec"`
	Status DeviceStatus `json:"status,omitempty"`
}

// DeviceSpec defines the desired state of a Device.
type DeviceSpec struct {
	// A simple, human-friendly, auto-incrementing integer ID.
	// Note: The logic for auto-incrementing this ID will need to be
	// implemented in a custom storage layer or reconciler.
	NumericID int `json:"numericId,omitempty"`

	// The type of hardware (e.g., "Node", "GPU", "Rack").
	ComponentType string `json:"componentType"`

	// A machine-readable identifier for the specific model of the hardware.
	DeviceTypeSlug string `json:"deviceTypeSlug,omitempty"`

	// The manufacturer name.
	Manufacturer string `json:"manufacturer,omitempty"`

	// The part number.
	PartNumber string `json:"partNumber,omitempty"`

	// The serial number.
	SerialNumber string `json:"serialNumber,omitempty"`

	// The ID of the physical location where the device is currently installed.
	LocationID string `json:"locationId,omitempty"`
}

// DeviceStatus defines the observed state of a Device.
type DeviceStatus struct {
	// A read-only list of devices contained within this one.
	ChildrenDeviceIDs []string `json:"childrenDeviceIds,omitempty"`

	// Represents the latest available observations of a resource's state.
	Conditions []resources.Condition `json:"conditions,omitempty"`
}
