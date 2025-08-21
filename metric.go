package metric

import (
	"errors"
	"expvar"
	"fmt"
	"sync"
	"time"
)

// Input represents a metric input source that provides measurements via its Collect method.
// Periodically called by the Collector to gather metrics.
type Input interface {
	Collect() (Measurement, error)
}

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func() (Measurement, error)

type Measurement struct {
	Name   string
	Fields []Field // name -value pairs and producer function
}

func (m *Measurement) AddField(f ...Field) {
	m.Fields = append(m.Fields, f...)
}

type Field struct {
	Name  string
	Value float64
	Unit  Unit
	Type  MakeProducerFunc
}

type MakeProducerFunc func() Producer

func FieldTypeCounter() Producer { return NewCounter() }
func FieldTypeGauge() Producer   { return NewGauge() }
func FieldTypeMeter() Producer   { return NewMeter() }

func FieldTypeHistogram(maxBin int, ps ...float64) func() Producer {
	return func() Producer {
		return NewHistogram(maxBin, ps...)
	}
}

type FieldInfo struct {
	Name   string
	Series string
	Type   MakeProducerFunc
	Unit   Unit
}

type InputWrapper struct {
	input          InputFunc
	measureName    string
	mtsFields      map[string]MultiTimeSeries
	publishedNames map[string]string
}

type EmitterWrapper struct {
	measureName    string
	mtsFields      map[string]MultiTimeSeries
	publishedNames map[string]string
}

type Collector struct {
	sync.Mutex
	// periodically collects metrics from inputs
	inputs   []*InputWrapper
	interval time.Duration
	closeCh  chan struct{}
	stopWg   sync.WaitGroup

	// event-driven emitters
	emitters   map[string]*EmitterWrapper
	recvCh     chan Measurement
	recvChSize int

	// time series configuration
	series       []CollectorSeries
	expvarPrefix string

	// persistent storage
	storage Storage
}

type CollectorSeries struct {
	name     string
	period   time.Duration
	maxCount int
}

// NewCollector creates a new Collector with the specified interval.
// The interval determines how often the inputs will be collected.
// The collector will run until Stop() is called.
// It is safe to call Start() multiple times, but Stop() should be called only once
func NewCollector(interval time.Duration, opts ...CollectorOption) *Collector {
	c := &Collector{
		interval: interval,
		closeCh:  make(chan struct{}),
		emitters: make(map[string]*EmitterWrapper),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.recvChSize <= 0 {
		c.recvChSize = 100
	}
	c.recvCh = make(chan Measurement, c.recvChSize)
	return c
}

type CollectorOption func(c *Collector)

func WithSeries(name string, period time.Duration, maxCount int) CollectorOption {
	return func(c *Collector) {
		c.series = append(c.series, CollectorSeries{name: name, period: period, maxCount: maxCount})
	}
}

func WithExpvarPrefix(prefix string) CollectorOption {
	return func(c *Collector) {
		c.expvarPrefix = prefix
	}
}

func WithReceiverSize(size int) CollectorOption {
	return func(c *Collector) {
		c.recvChSize = size
	}
}

func WithStorage(store Storage) CollectorOption {
	return func(c *Collector) {
		c.storage = store
	}
}

func (c *Collector) AddInput(input Input) {
	c.AddInputFunc(input.Collect)
}

func (c *Collector) AddInputFunc(input InputFunc) {
	c.Lock()
	defer c.Unlock()
	iw := &InputWrapper{
		input:          input,
		mtsFields:      make(map[string]MultiTimeSeries),
		publishedNames: make(map[string]string),
	}
	c.inputs = append(c.inputs, iw)
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
	c.stopWg.Add(1)
	go func() {
		defer c.stopWg.Done()
		for {
			select {
			case tm := <-ticker.C:
				c.runInputs(tm)
				c.syncStorage()
			case m := <-c.recvCh:
				c.receive(m)
			case <-c.closeCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (c *Collector) Stop() {
	close(c.closeCh)
	c.stopWg.Wait()
	close(c.recvCh)
	c.syncStorage()
}

func (c *Collector) runInputs(tm time.Time) {
	c.Lock()
	defer c.Unlock()

	for _, iw := range c.inputs {
		measure, err := iw.input()
		if err != nil {
			fmt.Printf("Error measuring: %v\n", err)
			continue
		}
		if iw.measureName == "" {
			iw.measureName = measure.Name
		}
		for _, field := range measure.Fields {
			var mts MultiTimeSeries
			if fm, exists := iw.mtsFields[field.Name]; exists {
				mts = fm
			} else {
				mts = c.makeMultiTimeSeries(measure.Name, field)
				iw.mtsFields[field.Name] = mts

				publishName := c.makePublishName(measure.Name, field.Name)
				expvar.Publish(publishName, MultiTimeSeries(mts))
				iw.publishedNames[field.Name] = publishName
			}
			mts.AddTime(tm, field.Value)
		}
	}
}

func (c *Collector) makePublishName(measureName, fieldName string) string {
	var prefix string
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	return fmt.Sprintf("%s%s:%s", prefix, measureName, fieldName)
}

// EventChannel returns a channel to which measurements can be sent.
// This channel is used by emitters to send measurements to the collector.
func (c *Collector) EventChannel() chan<- Measurement {
	return c.recvCh
}

// SendEvent processes a measurement sent to the collector.
func (c *Collector) SendEvent(m Measurement) {
	c.recvCh <- m
}

func (c *Collector) receive(m Measurement) {
	c.Lock()
	defer c.Unlock()

	now := nowFunc()

	emit, ok := c.emitters[m.Name]
	if !ok {
		emit = &EmitterWrapper{
			measureName:    m.Name,
			mtsFields:      make(map[string]MultiTimeSeries),
			publishedNames: make(map[string]string),
		}
		c.emitters[m.Name] = emit
	}
	for _, field := range m.Fields {
		var mts MultiTimeSeries
		if fm, exists := emit.mtsFields[field.Name]; exists {
			mts = fm
		} else {
			mts = c.makeMultiTimeSeries(m.Name, field)
			emit.mtsFields[field.Name] = mts

			publishName := c.makePublishName(m.Name, field.Name)
			expvar.Publish(publishName, MultiTimeSeries(mts))
			emit.publishedNames[field.Name] = publishName
		}
		mts.AddTime(now, field.Value)
	}
}

func (c *Collector) makeMultiTimeSeries(measureName string, field Field) MultiTimeSeries {
	mts := make(MultiTimeSeries, len(c.series))
	for i, ser := range c.series {
		var ts = NewTimeSeries(ser.period, ser.maxCount, field.Type())
		if c.storage != nil {
			seriesName := cleanPath(ts.interval.String())
			if data, err := c.storage.Load(measureName, field.Name, seriesName); err != nil {
				fmt.Printf("Failed to load time series for %s %s %s: %v\n", measureName, field.Name, ser.name, err)
			} else if data != nil {
				// if file is not exists, data will be nil
				ts.data = data.data
			}
		}
		mts[i] = ts
		mts[i].SetMeta(FieldInfo{
			Name:   field.Name,
			Series: ser.name,
			Type:   field.Type,
			Unit:   field.Unit,
		})
	}
	return mts
}

func (c *Collector) Names() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	for _, iw := range c.inputs {
		for _, publishedName := range iw.publishedNames {
			names = append(names, publishedName)
		}
	}
	return names
}

func (c *Collector) Snapshot(metricName string, seriesName string) (*Snapshot, error) {
	idx := -1
	for i, n := range c.series {
		if n.name == seriesName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, MetricNotFoundError
	}
	return snapshot(metricName, 0)
}

func (c *Collector) syncStorage() {
	if c.storage == nil {
		return
	}
	func() {
		c.Lock()
		defer c.Unlock()
		for _, iw := range c.inputs {
			for fieldName, mts := range iw.mtsFields {
				for _, ts := range mts {
					seriesName := cleanPath(ts.interval.String())
					err := c.storage.Store(iw.measureName, fieldName, seriesName, ts)
					if err != nil {
						fmt.Printf("Failed to store time series for %s %s %s: %v\n", iw.measureName, fieldName, seriesName, err)
					}
				}
			}
		}
	}()

	func() {
		c.Lock()
		defer c.Unlock()
		for measureName, emit := range c.emitters {
			for fieldName, mts := range emit.mtsFields {
				for _, ts := range mts {
					seriesName := cleanPath(ts.interval.String())
					err := c.storage.Store(measureName, fieldName, seriesName, ts)
					if err != nil {
						fmt.Printf("Failed to store time series for %s %s %s: %v\n", measureName, fieldName, seriesName, err)
					}
				}
			}
		}
	}()
}

type Output interface {
	Export(name string, data *Snapshot) error
}

type OutputWrapper struct {
	output Output
	filter func(string) bool
}

type Exporter struct {
	sync.Mutex
	ows       []OutputWrapper
	metrics   []string
	interval  time.Duration
	closeCh   chan struct{}
	latestErr error
}

func NewExporter(interval time.Duration, metrics []string) *Exporter {
	return &Exporter{
		interval: interval,
		metrics:  metrics,
		closeCh:  make(chan struct{}),
	}
}

func (s *Exporter) AddOutput(output Output, filter any) {
	s.Lock()
	defer s.Unlock()
	ow := OutputWrapper{
		output: output,
		filter: func(string) bool { return true }, // Default filter allows all metrics
	}
	s.ows = append(s.ows, ow)
}

func (s *Exporter) Start() {
	ticker := time.NewTicker(s.interval)
	go func() {
		for {
			select {
			case <-s.closeCh:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := s.exportAll(0); err != nil {
					s.latestErr = err
				}
			}
		}
	}()
}

func (s *Exporter) Stop() {
	if s.closeCh == nil {
		return
	}
	close(s.closeCh)
	s.closeCh = nil
}

// Err returns the latest error encountered during export.
// If no error has occurred, it returns nil.
func (s *Exporter) Err() error {
	return s.latestErr
}

func (s *Exporter) exportAll(tsIdx int) error {
	for _, metricName := range s.metrics {
		if err := s.Export(metricName, tsIdx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Exporter) Export(metricName string, tsIdx int) error {
	var ss *Snapshot
	var name string
	var data *Snapshot
	for _, ow := range s.ows {
		if !ow.filter(metricName) {
			continue
		}
		if ss == nil {
			var err error
			ss, err = snapshot(metricName, tsIdx)
			if err != nil {
				return err
			}
			if ss == nil || len(ss.Values) == 0 {
				// If the metric is nil or has no values, skip
				break
			}
			name = fmt.Sprintf("%s:%d", metricName, tsIdx)
			data = ss
		}
		if err := ow.output.Export(name, data); err != nil {
			return err
		}
	}
	return nil
}

func snapshot(metricName string, idx int) (*Snapshot, error) {
	if ev := expvar.Get(metricName); ev != nil {
		mts, ok := ev.(MultiTimeSeries)
		if !ok {
			return nil, fmt.Errorf("metric %s is not a Metric, but %T", metricName, ev)
		}
		if idx < 0 || idx >= len(mts) {
			return nil, fmt.Errorf("index %d out of range for metric %s with %d time series",
				idx, metricName, len(mts))
		}
		return mts[idx].Snapshot(), nil
	}
	return nil, MetricNotFoundError
}

var MetricNotFoundError = errors.New("metric not found")
