package nephila

import (
	"fmt"
)

func NewNephilaProvider(nephilaType string) (NephilaProvider, error) {

	if nephilaType == Flannel {
		var fnp FlannelNephilaProvider
		return fnp, nil

	}

	return nil, fmt.Errorf("[Azure CNS] Failed to determine Nephila type.")
}
