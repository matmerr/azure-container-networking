package nephila

import (
	"fmt"
)

// NewNephilaProvider returns a nephila provider based on the type parameter
func NewNephilaProvider(nephilaType string) (NephilaProvider, error) {
	if nephilaType == Disabled {
		var dnp DisabledNephilaProvider
		return &dnp, nil
	}
	if nephilaType == Flannel {
		var fnp FlannelNephilaProvider
		return &fnp, nil
	}
	return nil, fmt.Errorf("failed to determine Nephila provider type: %+v", nephilaType)
}
