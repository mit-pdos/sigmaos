package fss3

import (
	// "log"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInsert(t *testing.T) {
	ws := mkWindows()
	ws.insertw(&window{0, 10})
	ws.insertw(&window{10, 20})
	assert.Equal(t, 1, len(ws.ws))
	ws.insertw(&window{15, 20})
	assert.Equal(t, 1, len(ws.ws))
	ws.insertw(&window{30, 40})
	assert.Equal(t, 2, len(ws.ws))
	ws.insertw(&window{20, 25})
	assert.Equal(t, 2, len(ws.ws))
	ws.insertw(&window{50, 60})
	assert.Equal(t, 3, len(ws.ws))
	ws.insertw(&window{70, 80})
	assert.Equal(t, 4, len(ws.ws))
	ws.insertw(&window{40, 50})
	assert.Equal(t, 3, len(ws.ws))
	ws.insertw(&window{25, 30})
	assert.Equal(t, 2, len(ws.ws))
	ws.insertw(&window{60, 70})
	assert.Equal(t, 1, len(ws.ws))
}
