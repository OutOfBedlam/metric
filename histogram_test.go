package metric

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistogram(t *testing.T) {
	h := NewHistogram(100)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, 50.0, h.Quantile(0.50))
	require.Equal(t, 75.0, h.Quantile(0.75))
	require.Equal(t, 90.0, h.Quantile(0.90))
	require.Equal(t, 99.0, h.Quantile(0.99))
	require.Equal(t, 100.0, h.Quantile(0.999))
}

func TestHistogram50(t *testing.T) {
	h := NewHistogram(50)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, 49.5, h.Quantile(0.50))
	require.Equal(t, 75.5, h.Quantile(0.75))
	require.Equal(t, 89.5, h.Quantile(0.90))
	require.Equal(t, 99.5, h.Quantile(0.99))
	require.Equal(t, 99.5, h.Quantile(0.999))
}

func TestHistogramQuantiles(t *testing.T) {
	h := NewHistogram(100)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, []float64{75.0, 50.0, 90.0}, h.Quantiles(0.75, 0.50, 0.90))
}
