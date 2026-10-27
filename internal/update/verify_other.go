//go:build !windows

package update

import "github.com/131/vauth-desktop/v2/internal/i18n"

// Authenticode signatures exist only on Windows, the only released target.
func Authenticode(candidate, current string) error {
	return i18n.Errorf("err.windows.only")
}
