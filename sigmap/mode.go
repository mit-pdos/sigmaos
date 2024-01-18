package sigmap

import (
	"fmt"
)

func (m Tmode) String() string {
	return fmt.Sprintf("m %x", uint8(m&0xFF))
}
