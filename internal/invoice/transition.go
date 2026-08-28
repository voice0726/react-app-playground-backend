package invoice

import (
	"errors"
	"fmt"
)

var ErrTransition = errors.New("invalid invoice status transition")

func CanTransition(current, next string) error {
	if current == next || next == "cancelled" || (current == "draft" && next == "sent") || (current == "sent" && next == "paid") {
		return nil
	}
	return fmt.Errorf("%w: %s to %s", ErrTransition, current, next)
}
