package metric

import (
	"errors"
	"expvar"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func(*Gather)

// OutputFunc is a function type that processes the collected ProductData.
type OutputFunc func(Product)

type Gather struct {
	fields []Field
	ts     time.Time
	noop   bool
	errs   MultipleError
}

func (g *Gather) Add(name string, value float64, typ Type) {
	g.fields = append(g.fields, Field{Name: name, Value: value, Type: typ})
}

func (g *Gather) AddError(err error) {
	g.errs = append(g.errs, err)
}

func (g Gather) Err() error {
	if len(g.errs) == 0 {
		return nil
	} else if len(g.errs) == 1 {
		return g.errs[0]
	}
	return g.errs
}

type Field struct {
	Name  string
	Value float64
	Type  Type
}

// CounterType supports samples count, value (sum)
func CounterType(u Unit) Type {
	return Type{
		p: func() Producer { return NewCounter() },
		s: "counter",
		u: u,
	}
}

// GaugeType supports samples count, sum, value (last)
func GaugeType(u Unit) Type {
	return Type{
		p: func() Producer { return NewGauge() },
		s: "gauge",
		u: u,
	}
}

// MeterType supports samples count, sum, first, last, min, max
func MeterType(u Unit) Type {
	return Type{
		p: func() Producer { return NewMeter() },
		s: "meter",
		u: u,
	}
}

// OdometerType supports samples count, quantiles
func HistogramType(u Unit) Type {
	return HistogramTypePercentiles(u, 100, 0.5, 0.90, 0.99)
}

func OdometerType(u Unit) Type {
	return Type{
		p: func() Producer { return NewOdometer() },
		s: "odometer",
		u: u,
	}
}

func HistogramTypePercentiles(u Unit, maxBin int, ps ...float64) Type {
	return Type{
		p: func() Producer { return NewHistogram(maxBin, ps...) },
		s: "histogram",
		u: u,
	}
}

// TimerType supports samples count, total, min, max
func TimerType(u Unit) Type {
	return Type{
		p: func() Producer { return NewTimer() },
		s: "timer",
		u: u,
	}
}

type FieldInfo struct {
	Name   string
	Series string
	Period time.Duration
	Type   string
	Unit   Unit
}

type Collector struct {
	sync.Mutex

	inputs     []Input                    // registered input
	outputs    []Output                   // registered output
	timeseries map[string]MultiTimeSeries // field_name: multi-timeseries

	// periodically collects metrics from inputs
	samplingInterval time.Duration
	closeCh          chan struct{}
	stopWg           sync.WaitGroup

	// event-driven measurements
	recvCh     chan *Gather
	recvChSize int
	// a channel to which measurements can be sent.
	C chan<- *Gather

	// time series configuration
	series       []CollectorSeries
	expvarPrefix string

	// persistent storage
	storage Storage
}

type CollectorSeries struct {
	Name     string
	Period   time.Duration
	MaxCount int
}

// NewCollector creates a new Collector with the specified interval.
// The interval determines how often the inputs will be collected.
// The collector will run until Stop() is called.
// It is safe to call Start() multiple times, but Stop() should be called only once
func NewCollector(opts ...CollectorOption) *Collector {
	c := &Collector{
		samplingInterval: 10 * time.Second,
		closeCh:          make(chan struct{}),
		timeseries:       make(map[string]MultiTimeSeries),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.recvChSize <= 0 {
		c.recvChSize = 100
	}
	c.recvCh = make(chan *Gather, c.recvChSize)
	c.C = c.recvCh
	return c
}

type CollectorOption func(c *Collector)

// WithSamplingInterval sets the collection interval for the collector.
// Default is 10 seconds.
func WithSamplingInterval(interval time.Duration) CollectorOption {
	return func(c *Collector) {
		c.samplingInterval = interval
	}
}

func WithSeries(name string, period time.Duration, maxCount int) CollectorOption {
	return func(c *Collector) {
		c.series = append(c.series, CollectorSeries{Name: name, Period: period, MaxCount: maxCount})
	}
}

// WithPrefix sets the prefix for all published expvar metrics.
func WithPrefix(prefix string) CollectorOption {
	return func(c *Collector) {
		c.expvarPrefix = prefix
	}
}

// WithInputBuffer sets the size of the input buffer channel.
func WithInputBuffer(size int) CollectorOption {
	return func(c *Collector) {
		c.recvChSize = size
	}
}

func WithStorage(store Storage) CollectorOption {
	return func(c *Collector) {
		c.storage = store
	}
}

type Input interface {
	Gather(*Gather)
}

type Output interface {
	Process(Product)
}

type MultipleError []error

var _ error = MultipleError{}

func (me MultipleError) Error() string {
	var sb strings.Builder
	for i, err := range me {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

func (c *Collector) AddOutput(outputs ...Output) error {
	var errs MultipleError
	c.Lock()
	defer c.Unlock()
	for _, out := range outputs {
		if hasInit, ok := out.(interface{ Init() error }); ok {
			if err := hasInit.Init(); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		c.outputs = append(c.outputs, out)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type OutputFuncWrapper struct {
	f OutputFunc
}

func (ow *OutputFuncWrapper) Process(p Product) {
	ow.f(p)
}

// AddOutputFunc adds an output function to the collector.
// The output function will be called with the collected Product.
func (c *Collector) AddOutputFunc(output OutputFunc) {
	c.outputs = append(c.outputs, &OutputFuncWrapper{output})
}

func (c *Collector) AddInput(inputs ...Input) error {
	var errs MultipleError
	var initialGathers []*Gather
	c.Lock()
	ts := nowFunc()
	defer func() {
		c.Unlock()
		for _, g := range initialGathers {
			g.ts = ts
			c.receive(g)
		}
	}()
	for _, input := range inputs {
		if hasInit, ok := input.(interface{ Init() error }); ok {
			if err := hasInit.Init(); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		// the first call to get the measurement name
		g := &Gather{}
		input.Gather(g)
		if err := g.Err(); err != nil {
			errs = append(errs, err)
			continue
		}
		initialGathers = append(initialGathers, g)
		c.inputs = append(c.inputs, input)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type InputFuncWrapper struct {
	f InputFunc
}

func (iw *InputFuncWrapper) Gather(g *Gather) {
	iw.f(g)
}

// AddInputFunc adds an input function to the collector.
func (c *Collector) AddInputFunc(input InputFunc) error {
	return c.AddInput(&InputFuncWrapper{f: input})
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.samplingInterval)
	c.stopWg.Add(1)
	go func() {
		defer c.stopWg.Done()
		for {
			select {
			case ts := <-ticker.C:
				go c.runInputs(ts)
			case m := <-c.recvCh:
				c.receive(m)
			case <-c.closeCh:
				ticker.Stop()
				// derain the recvCh
				for {
					select {
					case m := <-c.recvCh:
						c.receive(m)
					default:
						return
					}
				}
			}
		}
	}()
}

func (c *Collector) Stop() {
	close(c.closeCh)
	c.stopWg.Wait()
	close(c.recvCh)
	c.syncStorage()
	// call DeInit() of inputs if exists
	for _, input := range c.inputs {
		if hasDeInit, ok := input.(interface{ DeInit() error }); ok {
			hasDeInit.DeInit()
		}
	}
	// call DeInit() of outputs if exists
	for _, out := range c.outputs {
		if hasDeInit, ok := out.(interface{ DeInit() }); ok {
			hasDeInit.DeInit()
		}
	}
}

func (c *Collector) makePublishName(fieldName string) string {
	var prefix string
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	return fmt.Sprintf("%s%s", prefix, fieldName)
}

// Send processes a measurement sent to the collector.
func (c *Collector) Send(fields ...Field) {
	g := &Gather{
		fields: fields,
		ts:     nowFunc(),
	}
	c.recvCh <- g
}

func (c *Collector) runInputs(ts time.Time) {
	for _, input := range c.inputs {
		gather := &Gather{}
		input.Gather(gather)
		if err := gather.Err(); err != nil {
			fmt.Printf("Error measuring: %v\n", err)
			continue
		}
		gather.ts = ts
		// TODO: there are chances that recvCh is already closed
		// because of Stop() has been called.
		c.recvCh <- gather
	}
	c.recvCh <- &Gather{noop: true, ts: ts}
}

func (c *Collector) receive(m *Gather) {
	c.Lock()
	defer c.Unlock()

	if m.ts.IsZero() {
		m.ts = nowFunc()
	}

	if m.noop {
		nan := math.NaN()
		for _, mts := range c.timeseries {
			for _, ts := range mts {
				ts.AddTime(m.ts, nan)
			}
		}
		return
	}

	for _, field := range m.fields {
		var mts MultiTimeSeries
		if fm, exists := c.timeseries[field.Name]; exists {
			mts = fm
		} else {
			mts = c.makeMultiTimeSeries(field)
			c.timeseries[field.Name] = mts
			publishName := c.makePublishName(field.Name)
			expvar.Publish(publishName, mts)
		}
		mts.AddTime(m.ts, field.Value)
	}
}

type Product struct {
	Name   string        `json:"name"`
	Time   time.Time     `json:"ts"`
	Value  Value         `json:"value,omitempty"`
	IsNull bool          `json:"isNull,omitempty"`
	Series string        `json:"series"`
	Period time.Duration `json:"period"`
	Type   string        `json:"type"`
	Unit   Unit          `json:"unit"`
}

func (c *Collector) onProduct(tb TimeBin, meta any) {
	if len(c.outputs) == 0 {
		return
	}

	field, ok := meta.(FieldInfo)
	if !ok {
		return
	}

	data := Product{
		Name:   field.Name,
		Time:   tb.Time,
		Value:  tb.Value,
		IsNull: tb.IsNull,
		Series: field.Series,
		Period: field.Period,
		Type:   field.Type,
		Unit:   field.Unit,
	}
	for _, out := range c.outputs {
		out.Process(data)
	}
}

func (c *Collector) makeMultiTimeSeries(field Field) MultiTimeSeries {
	mts := make(MultiTimeSeries, len(c.series))
	for i, ser := range c.series {
		var ts = NewTimeSeries(ser.Period, ser.MaxCount, field.Type.Producer())
		ts.SetListener(c.onProduct)
		ts.SetMeta(FieldInfo{
			Name:   field.Name,
			Series: ser.Name,
			Period: ser.Period,
			Type:   field.Type.String(),
			Unit:   field.Type.Unit(),
		})
		if c.storage != nil {
			seriesName := cleanPath(ts.interval.String())
			if data, err := c.storage.Load(field.Name, seriesName); err != nil {
				fmt.Printf("Failed to load time series for %s %s: %v\n", field.Name, ser.Name, err)
			} else if data != nil {
				// if file is not exists, data will be nil
				ts.data = data.data
			}
		}
		mts[i] = ts
	}
	return mts
}

func (c *Collector) SamplingInterval() time.Duration {
	return c.samplingInterval
}

// PublishNames returns a list of all published metric names in the collector.
func (c *Collector) PublishNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	prefix := ""
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	for name := range c.timeseries {
		names = append(names, prefix+name)
	}
	return names
}

func (c *Collector) MetricNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	for name := range c.timeseries {
		names = append(names, name)
	}
	return names
}

// Timeseries returns the MultiTimeSeries for the specified field name.
// If the field does not exist, it returns nil.
func (c *Collector) Timeseries(name string) MultiTimeSeries {
	c.Lock()
	defer c.Unlock()
	return c.timeseries[name]
}

func (c *Collector) Series() []CollectorSeries {
	c.Lock()
	defer c.Unlock()
	ret := make([]CollectorSeries, len(c.series))
	copy(ret, c.series)
	return ret
}

// Inflight returns the current collecting data for each series of the specified measure and field.
// The key of the returned map is the series name.
// If the measure or field does not exist, it returns ErrMetricNotFound.
func (c *Collector) Inflight(field string) (map[string]Product, error) {
	var mts MultiTimeSeries
	if m, ok := c.timeseries[field]; !ok {
		return nil, ErrMetricNotFound
	} else {
		mts = m
	}

	ret := map[string]Product{}
	for idx, n := range c.series {
		seriesName := n.Name
		nfo, ok := mts[idx].Meta().(FieldInfo)
		if !ok {
			return nil, fmt.Errorf("metric %s series %s meta is not FieldInfo, but %T",
				field, seriesName, mts[idx].Meta())
		}
		ts, prd := mts[idx].Last()
		ret[seriesName] = Product{
			Name:   nfo.Name,
			Time:   ts,
			Value:  prd,
			IsNull: prd == nil,
			Series: nfo.Series,
			Type:   nfo.Type,
			Unit:   nfo.Unit,
		}
	}
	return ret, nil
}

func (c *Collector) syncStorage() {
	if c.storage == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	for name, mts := range c.timeseries {
		for _, ts := range mts {
			seriesName := cleanPath(ts.interval.String())
			err := c.storage.Store(name, seriesName, ts)
			if err != nil {
				fmt.Printf("Failed to store time series for %s %s %s: %v\n", name, ts.Meta(), seriesName, err)
			}
		}
	}
}

var ErrMetricNotFound = errors.New("metric not found")
