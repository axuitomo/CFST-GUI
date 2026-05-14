package probecore

import (
	"fmt"
	"sort"
	"strings"
)

const PortPolicySourceOverrideGlobal = "source_override_global"

type TaskContext struct {
	ConfigSource     string `json:"config_source"`
	CurrentTestPort  int    `json:"current_test_port"`
	GlobalTCPPort    int    `json:"global_tcp_port"`
	PortPolicy       string `json:"port_policy"`
	SourcePortValues []int  `json:"source_port_values"`
}

type PortGroup struct {
	Port int
	IPs  []string
}

func TaskContextForPorts(globalPort int, sourcePorts map[string]int) (TaskContext, []string) {
	globalPort = normalizeGlobalPort(globalPort)
	values := UniquePortValues(sourcePorts)
	currentPort := globalPort
	warnings := make([]string, 0)
	if len(values) == 1 {
		currentPort = values[0]
	} else if len(values) > 1 {
		currentPort = 0
		warnings = append(warnings, fmt.Sprintf("输入源包含多个端口 %v，将按端口分组执行；未声明端口的候选使用全局测速端口 %d。", values, globalPort))
	}
	return TaskContext{
		CurrentTestPort:  currentPort,
		GlobalTCPPort:    globalPort,
		PortPolicy:       PortPolicySourceOverrideGlobal,
		SourcePortValues: values,
	}, warnings
}

func PortGroups(ips []string, sourcePorts map[string]int, globalPort int) []PortGroup {
	globalPort = normalizeGlobalPort(globalPort)
	groupsByPort := make(map[int][]string)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		port := sourcePorts[ip]
		if port <= 0 {
			port = globalPort
		}
		groupsByPort[port] = append(groupsByPort[port], ip)
	}
	ports := make([]int, 0, len(groupsByPort))
	for port := range groupsByPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	groups := make([]PortGroup, 0, len(ports))
	for _, port := range ports {
		groups = append(groups, PortGroup{Port: port, IPs: groupsByPort[port]})
	}
	return groups
}

func PortGroupPorts(groups []PortGroup) []int {
	ports := make([]int, 0, len(groups))
	for _, group := range groups {
		if group.Port > 0 {
			ports = append(ports, group.Port)
		}
	}
	return ports
}

func PortSummary(entries []string, sourcePorts map[string]int, globalPort int) map[string]any {
	globalPort = normalizeGlobalPort(globalPort)
	groups := PortGroups(entries, sourcePorts, globalPort)
	groupedPorts := PortGroupPorts(groups)
	currentPort := globalPort
	if len(groupedPorts) == 1 {
		currentPort = groupedPorts[0]
	} else if len(groupedPorts) > 1 {
		currentPort = 0
	}
	return map[string]any{
		"current_test_port":  currentPort,
		"grouped_ports":      groupedPorts,
		"global_tcp_port":    globalPort,
		"port_policy":        PortPolicySourceOverrideGlobal,
		"source_port_values": SourcePortValuesForEntries(entries, sourcePorts),
	}
}

func EffectivePortForSourcePorts(sourcePorts map[string]int, globalPort int) int {
	values := UniquePortValues(sourcePorts)
	if len(values) == 1 {
		return values[0]
	}
	return normalizeGlobalPort(globalPort)
}

func UniquePortValues(sourcePorts map[string]int) []int {
	if len(sourcePorts) == 0 {
		return []int{}
	}
	seen := make(map[int]struct{}, len(sourcePorts))
	values := make([]int, 0, len(sourcePorts))
	for _, port := range sourcePorts {
		if port <= 0 {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		values = append(values, port)
	}
	sort.Ints(values)
	return values
}

func SourcePortValuesForEntries(entries []string, sourcePorts map[string]int) []int {
	if len(entries) == 0 || len(sourcePorts) == 0 {
		return []int{}
	}
	ports := make(map[string]int, len(sourcePorts))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if port := sourcePorts[entry]; port > 0 {
			ports[entry] = port
		}
	}
	return UniquePortValues(ports)
}

func CloneStringIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func PrunePortsToEntries(sourcePorts map[string]int, entries []string) map[string]int {
	if len(sourcePorts) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			allowed[entry] = struct{}{}
		}
	}
	pruned := make(map[string]int, len(sourcePorts))
	for token, port := range sourcePorts {
		if _, ok := allowed[token]; ok && port > 0 {
			pruned[token] = port
		}
	}
	if len(pruned) == 0 {
		return nil
	}
	return pruned
}

func normalizeGlobalPort(port int) int {
	if port <= 0 {
		return 443
	}
	return port
}
