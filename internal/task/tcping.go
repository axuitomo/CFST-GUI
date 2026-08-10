package task

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	defaultTCPConnectTimeout = time.Second * 1
	maxRoutine               = 1000
	MinPingTimes             = 2
	defaultRoutines          = 200
	defaultPort              = 443
	defaultPingTimes         = 4
)

type Ping struct {
	engine   *Engine
	wg       *sync.WaitGroup
	m        *sync.Mutex
	ips      []*net.IPAddr
	csv      utils.PingDelaySet
	control  chan bool
	bar      *utils.Bar
	tcpProbe func(*net.IPAddr) (bool, time.Duration)
	passed   atomic.Int32
	total    int
	done     atomic.Int32
}

func (p *Ping) probeEngine() *Engine {
	if p.engine == nil {
		p.engine = NewEngine(DefaultConfig(), Hooks{})
	}
	return p.engine
}

func (e *Engine) NewPing() (*Ping, error) {
	ips, err := loadIPRangesConfig(e.config)
	if err != nil {
		return nil, err
	}
	return &Ping{
		engine:  e,
		wg:      &sync.WaitGroup{},
		m:       &sync.Mutex{},
		ips:     ips,
		csv:     make(utils.PingDelaySet, 0),
		control: make(chan bool, e.config.Routines),
		bar:     utils.NewBar(len(ips), "可用:", ""),
		total:   len(ips),
	}, nil
}

func (p *Ping) Run() utils.PingDelaySet {
	if len(p.ips) == 0 {
		return p.csv
	}
	config := p.probeEngine().config
	if config.Httping {
		utils.Cyan.Printf("开始延迟测速（模式：HTTP, 端口：%d, 范围：%v ~ %v ms, 丢包：%.2f)\n", config.TCPPort, config.InputMinDelay.Milliseconds(), config.InputMaxDelay.Milliseconds(), config.InputMaxLossRate)
	} else {
		utils.Cyan.Printf("开始延迟测速（模式：TCP, 端口：%d, 范围：%v ~ %v ms, 丢包：%.2f)\n", config.TCPPort, config.InputMinDelay.Milliseconds(), config.InputMaxDelay.Milliseconds(), config.InputMaxLossRate)
	}
	for _, ip := range p.ips {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			break
		}
		p.wg.Add(1)
		p.control <- false
		go p.start(ip)
	}
	p.wg.Wait()
	p.bar.Done()
	sort.Sort(p.csv)
	return p.csv
}

func (p *Ping) start(ip *net.IPAddr) {
	defer p.wg.Done()
	p.probeEngine().checkPause("stage1_tcp", ip.String())
	if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
		<-p.control
		return
	}
	p.tcpingHandler(ip)
	<-p.control
}

// bool connectionSucceed float32 time
func (p *Ping) tcping(ip *net.IPAddr) (bool, time.Duration) {
	startTime := time.Now()
	var fullAddress string
	if isIPv4(ip.String()) {
		fullAddress = fmt.Sprintf("%s:%d", ip.String(), p.probeEngine().config.TCPPort)
	} else {
		fullAddress = fmt.Sprintf("[%s]:%d", ip.String(), p.probeEngine().config.TCPPort)
	}
	ctx, cancel := context.WithTimeout(p.probeEngine().context(), p.probeEngine().config.TCPConnectTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", fullAddress)
	if err != nil {
		return false, 0
	}
	defer conn.Close()
	duration := time.Since(startTime)
	return true, duration
}

func (p *Ping) tcpProbeOnce(ip *net.IPAddr) (bool, time.Duration) {
	var ok bool
	var delay time.Duration
	for attempt := 1; attempt <= p.probeEngine().retryAttemptLimit(); attempt++ {
		if p.tcpProbe != nil {
			ok, delay = p.tcpProbe(ip)
		} else {
			ok, delay = p.tcping(ip)
		}
		if ok {
			return true, delay
		}
		if attempt < p.probeEngine().retryAttemptLimit() {
			p.probeEngine().sleepBeforeRetry("stage1_tcp", ip.String(), attempt)
		}
	}
	return false, 0
}

// pingReceived pingTotalTime
func (p *Ping) checkConnection(ip *net.IPAddr) (sent, recv int, totalDelay time.Duration, colo string) {
	if p.probeEngine().config.Httping {
		recv, totalDelay, colo = p.httping(ip)
		sent = p.probeEngine().config.PingTimes
		return
	}
	colo = "" // TCPing 不获取 colo
	if p.probeEngine().config.SkipFirstLatencySample {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			return
		}
		_, _ = p.tcpProbeOnce(ip)
	}
	for i := 0; i < p.probeEngine().config.PingTimes; i++ {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			return
		}
		ok, delay := p.tcpProbeOnce(ip)
		sent++
		if ok {
			recv++
			totalDelay += delay
		}
	}
	return
}

func (p *Ping) appendIPData(data *utils.PingData) {
	p.m.Lock()
	defer p.m.Unlock()
	p.csv = append(p.csv, utils.CloudflareIPData{
		PingData: data,
	})
}

// handle tcping
func (p *Ping) tcpingHandler(ip *net.IPAddr) {
	sent, recv, totalDlay, colo := p.checkConnection(ip)
	if recv != 0 {
		p.passed.Add(1)
	}
	p.probeEngine().noteStageProbeOutcome("stage1_tcp", ip.String(), recv != 0)
	processed := int(p.done.Add(1))
	nowAble := int(p.passed.Load())
	p.bar.Grow(1, strconv.Itoa(nowAble))
	if p.probeEngine().hooks.LatencyProgress != nil {
		p.probeEngine().hooks.LatencyProgress(processed, nowAble, processed-nowAble, p.total)
	}
	if recv == 0 {
		p.probeEngine().debugEvent("stage.reject", map[string]any{
			"ip":      ip.String(),
			"message": "TCP 测延迟未获得成功样本，淘汰该 IP。",
			"reason":  "tcp_no_response",
			"stage":   "stage1_tcp",
			"tcp": map[string]any{
				"received": recv,
				"sent":     sent,
			},
		})
		return
	}
	data := &utils.PingData{
		IP:       ip,
		Sended:   sent,
		Received: recv,
		Delay:    totalDlay / time.Duration(recv),
		Colo:     colo,
	}
	p.appendIPData(data)
}
