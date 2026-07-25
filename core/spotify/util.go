package spotify

import (
	"fmt"
)

// wrapErr — small helper to keep error messages consistent
func wrapErr(ctx string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", ctx, err)
}
