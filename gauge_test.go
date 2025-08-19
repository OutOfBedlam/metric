package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGaugeJSON(t *testing.T) {
	g := NewGauge()
	g.Add(1.0)
	g.Add(2.0)
	g.Add(3.0)

	data, err := json.Marshal(g)
	require.NoError(t, err)

	expected := `{"count":3,"sum":6,"value":3}`
	require.JSONEq(t, expected, string(data))

	var g2 Gauge
	err = json.Unmarshal(data, &g2)
	require.NoError(t, err)

	require.Equal(t, g.count, g2.count)
	require.Equal(t, g.sum, g2.sum)
	require.Equal(t, g.value, g2.value)
}
