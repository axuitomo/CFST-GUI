// 探测参数（数值三元组/超时/阈值）。
export interface ProbeNumericTriple {
  stage1: number;
  stage2: number;
  stage3: number;
}

export interface ProbeStageLimits {
  stage1?: number;
  stage2: number;
  stage3: number;
}

export interface ProbeTimeouts {
  stage1_ms: number;
  stage2_ms: number;
}

export interface ProbeThresholds {
  max_tcp_latency_ms: number | null;
  min_download_mbps: number;
}
