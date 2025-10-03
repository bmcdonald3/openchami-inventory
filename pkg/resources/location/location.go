package location

import "github.com/openchami/inventory/pkg/resources"

// Location represents a physical location where hardware can be installed.
type Location struct {
	resources.Resource
	Spec   LocationSpec   `json:"spec"`
	Status LocationStatus `json:"status,omitempty"`
}

// LocationSpec defines the desired state of a Location.
type LocationSpec struct {
	// A simple, human-friendly, auto-incrementing integer ID.
	NumericID int `json:"numericId,omitempty"`

	// The ID of the location this one is inside of.
	ParentLocationID string `json:"parentLocationId,omitempty"`

	// The type of physical space (e.g., "Rack", "Chassis", "Slot").
	LocationType string `json:"locationType"`
}

// LocationStatus defines the observed state of a Location.
type LocationStatus struct {
	// A read-only list of locations contained within this one.
	ChildrenLocationIDs []string `json:"childrenLocationIds,omitempty"`

	// Represents the latest available observations of a resource's state.
	Conditions []resources.Condition `json:"conditions,omitempty"`
}
