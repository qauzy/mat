//go:build !(android && cmfa)

package process

import "github.com/qauzy/mat/constant"

func FindPackageName(metadata *constant.Metadata) (string, error) {
	return "", nil
}
