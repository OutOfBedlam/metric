
## Metrics

### Counter

```go
c := &Counter{}
c.Mark(10)
fmt.Println(c.String())

c.Mark(5)
fmt.Println(c.String())

c.Reset()
fmt.Println(c.String())

// Output:
// 10
// 15
// 0
```

### Gauge

```go
g := &Gauge{}
g.Mark(42.1)
fmt.Println(g.String())
g.Mark(3.1415)
fmt.Println(g.String())
g.Reset()
fmt.Println(g.String())

// Output:
// 42.1
// 3.1415
// 0
```

### Meter

- Extension Points

```go
const(
	MeterExtendValue MeterExtendPoint = iota
	MeterExtendMin
	MeterExtendMax
	MeterExtendSum
	MeterExtendCount
	MeterExtendAvg
)
```

- Example

```go
m := &Meter{}
m.Mark(42.1)
fmt.Printf("%+v\n", m.Snapshot())
m.Mark(3.1415)
fmt.Printf("%+v\n", m.Snapshot())
m.Reset()
fmt.Printf("%+v\n", m.Snapshot())

// Output:
// {Value:42.1 Count:1 Sum:42.1 Min:42.1 Max:42.1}
// {Value:3.1415 Count:2 Sum:45.2415 Min:3.1415 Max:42.1}
// {Value:0 Count:0 Sum:0 Min:0 Max:0}
```

### Timer

- Extension Points

```go
const(
	TimerExtendValue TimerExtendPoint = iota
	TimerExtendCount
	TimerExtendTotal
	TimerExtendMin
	TimerExtendMax
	TimerExtendAvg
)
```

- Example
```go
timer := &Timer{}

// Simulate some work
for range 10 {
    tick := time.Now()
    time.Sleep(100*time.Millisecond)
}

timer.Mark(time.Since(tick))

s := timer.Snapshot()
fmt.Printf("%+v\n", s)

// Output:
// {Count:2 TotalDuration:1.5s MinDuration:400ms MaxDuration:1.1s}
```

## Extension

### Histogram

```go
h := NewHistogram(100)

for i := 1; i <= 100; i++ {
    h.Add(float64(i))
}

require.Equal(t, []float64{75.0, 50.0, 90.0}, h.Quantiles(0.75, 0.50, 0.90))
```

### TimeSeries