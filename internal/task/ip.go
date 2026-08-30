package task

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultInputFile = "ip.txt"

func isIPv4(ip string) bool {
	return strings.Contains(ip, ".")
}

func randIPEndWith(num byte) byte {
	if num == 0 { // 对于 /32 这种单独的 IP
		return byte(0)
	}
	return byte(rand.Intn(int(num)))
}

type IPRanges struct {
	testAll bool
	ips     []*net.IPAddr
	mask    string
	firstIP net.IP
	ipNet   *net.IPNet
}

func newIPRanges(testAll bool) *IPRanges {
	return &IPRanges{
		testAll: testAll,
		ips:     make([]*net.IPAddr, 0),
	}
}

// 如果是单独 IP 则加上子网掩码，反之则获取子网掩码(r.mask)
func (r *IPRanges) fixIP(ip string) string {
	// 如果不含有 '/' 则代表不是 IP 段，而是一个单独的 IP，因此需要加上 /32 /128 子网掩码
	if i := strings.IndexByte(ip, '/'); i < 0 {
		if isIPv4(ip) {
			r.mask = "/32"
		} else {
			r.mask = "/128"
		}
		ip += r.mask
	} else {
		r.mask = ip[i:]
	}
	return ip
}

// 解析 IP 段，获得 IP、IP 范围、子网掩码
func (r *IPRanges) parseCIDR(ip string) error {
	var err error
	if r.firstIP, r.ipNet, err = net.ParseCIDR(r.fixIP(ip)); err != nil {
		return fmt.Errorf("ParseCIDR err: %w", err)
	}
	return nil
}

func (r *IPRanges) appendIPv4(d byte) {
	r.appendIP(net.IPv4(r.firstIP[12], r.firstIP[13], r.firstIP[14], d))
}

func (r *IPRanges) appendIP(ip net.IP) {
	r.ips = append(r.ips, &net.IPAddr{IP: ip})
}

// 返回第四段 ip 的最小值及可用数目
func (r *IPRanges) getIPRange() (minIP, hosts byte) {
	minIP = r.firstIP[15] & r.ipNet.Mask[3] // IP 第四段最小值

	// 根据子网掩码获取主机数量
	m := net.IPv4Mask(255, 255, 255, 255)
	for i, v := range r.ipNet.Mask {
		m[i] ^= v
	}
	total, _ := strconv.ParseInt(m.String(), 16, 32) // 总可用 IP 数
	if total > 255 {                                 // 矫正 第四段 可用 IP 数
		hosts = 255
		return
	}
	hosts = byte(total)
	return
}

func (r *IPRanges) chooseIPv4() {
	if r.mask == "/32" { // 单个 IP 则无需随机，直接加入自身即可
		r.appendIP(r.firstIP)
	} else {
		minIP, hosts := r.getIPRange()    // 返回第四段 IP 的最小值及可用数目
		for r.ipNet.Contains(r.firstIP) { // 只要该 IP 没有超出 IP 网段范围，就继续循环随机
			if r.testAll { // 如果是测速全部 IP
				for i := 0; i <= int(hosts); i++ { // 遍历 IP 最后一段最小值到最大值
					r.appendIPv4(byte(i) + minIP)
				}
			} else { // 随机 IP 的最后一段 0.0.0.X
				r.appendIPv4(minIP + randIPEndWith(hosts))
			}
			r.firstIP[14]++ // 0.0.(X+1).X
			if r.firstIP[14] == 0 {
				r.firstIP[13]++ // 0.(X+1).X.X
				if r.firstIP[13] == 0 {
					r.firstIP[12]++ // (X+1).X.X.X
				}
			}
		}
	}
}

func (r *IPRanges) chooseIPv6() {
	if r.mask == "/128" { // 单个 IP 则无需随机，直接加入自身即可
		r.appendIP(r.firstIP)
	} else {
		for i := 8; i < net.IPv6len; i++ {
			randomHost := randIPEndWith(255)
			r.firstIP[i] = (r.firstIP[i] & r.ipNet.Mask[i]) | (randomHost &^ r.ipNet.Mask[i])
		}

		for r.ipNet.Contains(r.firstIP) {
			targetIP := append(net.IP(nil), r.firstIP...)
			r.appendIP(targetIP)

			for i := 7; i >= 0; i-- {
				previous := r.firstIP[i]
				r.firstIP[i] += byte(rand.Intn(255) + 1)
				if r.firstIP[i] >= previous {
					break
				}
			}
		}
	}
}

func loadIPRangesConfig(config Config) ([]*net.IPAddr, error) {
	ranges := newIPRanges(config.TestAll)
	if config.IPText != "" { // 从参数中获取 IP 段数据
		IPs := strings.Split(config.IPText, ",") // 以逗号分隔为数组并循环遍历
		for _, IP := range IPs {
			IP = strings.TrimSpace(IP) // 去除首尾的空白字符（空格、制表符、换行符等）
			if IP == "" {              // 跳过空的（即开头、结尾或连续多个 ,, 的情况）
				continue
			}
			if err := ranges.parseCIDR(IP); err != nil { // 解析 IP 段，获得 IP、IP 范围、子网掩码
				return nil, err
			}
			if isIPv4(IP) { // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
	} else { // 从文件中获取 IP 段数据
		if config.IPFile == "" {
			config.IPFile = defaultInputFile
		}
		file, err := os.Open(config.IPFile)
		if err != nil {
			return nil, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() { // 循环遍历文件每一行
			line := strings.TrimSpace(scanner.Text()) // 去除首尾的空白字符（空格、制表符、换行符等）
			if line == "" {                           // 跳过空行
				continue
			}
			if err := ranges.parseCIDR(line); err != nil { // 解析 IP 段，获得 IP、IP 范围、子网掩码
				return nil, err
			}
			if isIPv4(line) { // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
	}
	return ranges.ips, nil
}
