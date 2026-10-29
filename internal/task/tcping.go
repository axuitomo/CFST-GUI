package task

import (
	"context"
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
	MinPingTimes             = 2
	defaultRoutines          = 200
	defaultPort              = 443
	defaultPingTimes         = 4
	// TLS 握手要多个 RTT，不能沿用阶段 1 的 TCP 建连预算。
	defaultTLSHandshakeTimeout = time.Second * 3
)

// tlsHandshakeTimeout 给 TLS 单独留预算：阶段 1 的 TCP 超时会被用户调小用来筛快 IP，
// 直接沿用会让远端边缘的握手超时，且失败会计入可重试的请求错误。
func tlsHandshakeTimeout(connectTimeout time.Duration) time.Duration {
	if connectTimeout*2 > defaultTLSHandshakeTimeout {
		return connectTimeout * 2
	}
	return defaultTLSHandshakeTimeout
}

func defaultRoutinesForPlatform(goos string) int {
	if goos == "android" {
		return 64
	}
	return defaultRoutines
}

type Ping struct {
	engine   *Engine
	wg       *sync.WaitGroup
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
	results := make(utils.PingDelaySet, len(p.ips))
	passed := make([]bool, len(p.ips))
	for index, ip := range p.ips {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			break
		}
		p.wg.Add(1)
		p.control <- false
		go p.start(index, ip, results, passed)
	}
	p.wg.Wait()
	p.bar.Done()
	kept := 0
	for index, ok := range passed {
		if ok {
			results[kept] = results[index]
			kept++
		}
	}
	p.csv = results[:kept]
	sort.Sort(p.csv)
	return p.csv
}

func (p *Ping) start(index int, ip *net.IPAddr, results utils.PingDelaySet, passed []bool) {
	defer p.wg.Done()
	engine := p.probeEngine()
	// 信号量在 panic 路径也必须释放，否则后面所有 IP 都会卡在 p.control <- false 上。
	defer func() { <-p.control }()
	defer engine.recoverWorkerPanic("stage1_tcp", ip.String(), "tcp_worker_panic")
	setWorkerLabels("stage1_tcp", ip.String())
	engine.checkPause("stage1_tcp", ip.String())
	if engine.isCanceled("stage1_tcp", ip.String()) {
		return
	}
	p.tcpingHandler(index, ip, results, passed)
}

// probeAddress builds the dial target once per IP instead of per attempt.
func (p *Ping) probeAddress(ip *net.IPAddr) string {
	return net.JoinHostPort(ip.String(), strconv.Itoa(p.probeEngine().config.TCPPort))
}

func (p *Ping) tcping(address string) (bool, time.Duration) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(p.probeEngine().context(), p.probeEngine().config.TCPConnectTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return false, 0
	}
	defer conn.Close()
	duration := time.Since(startTime)
	return true, duration
}

func (p *Ping) tcpProbeOnce(ip *net.IPAddr, address string) (bool, time.Duration) {
	var ok bool
	var delay time.Duration
	for attempt := 1; attempt <= p.probeEngine().retryAttemptLimit(); attempt++ {
		if p.tcpProbe != nil {
			ok, delay = p.tcpProbe(ip)
		} else {
			ok, delay = p.tcping(address)
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
func (p *Ping) checkConnection(ip *net.IPAddr) (sent, recv int, totalDelay time.Duration, colo, note string) {
	if p.probeEngine().config.Httping {
		recv, totalDelay, colo, note = p.httping(ip)
		sent = p.probeEngine().config.PingTimes
		return
	}
	colo = "" // TCPing 不获取 colo
	address := p.probeAddress(ip)
	if p.probeEngine().config.SkipFirstLatencySample {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			return
		}
		_, _ = p.tcpProbeOnce(ip, address)
	}
	for i := 0; i < p.probeEngine().config.PingTimes; i++ {
		p.probeEngine().checkPause("stage1_tcp", ip.String())
		if p.probeEngine().isCanceled("stage1_tcp", ip.String()) {
			return
		}
		ok, delay := p.tcpProbeOnce(ip, address)
		sent++
		if ok {
			recv++
			totalDelay += delay
		}
	}
	return
}

// handle tcping
func (p *Ping) tcpingHandler(index int, ip *net.IPAddr, results utils.PingDelaySet, passed []bool) {
	sent, recv, totalDlay, colo, note := p.checkConnection(ip)
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
		if p.probeEngine().debugEnabled() {
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
		}
		return
	}
	data := &utils.PingData{
		IP:       ip,
		Sended:   sent,
		Received: recv,
		Delay:    totalDlay / time.Duration(recv),
		Colo:     colo,
		Note:     note,
	}
	results[index] = utils.CloudflareIPData{PingData: data}
	passed[index] = true
}
