import type { ColoFilterMode, SourceIPMode, SourceKind } from "./enums";

export interface SourceConfig {
  colo_filter: string;
  colo_filter_mode: ColoFilterMode;
  content: string;
  enabled: boolean;
  id: string;
  ip_limit: number;
  ip_mode: SourceIPMode;
  kind: SourceKind;
  last_fetched_at: string;
  last_fetched_count: number;
  name: string;
  path: string;
  status_text: string;
  url: string;
}

export interface SourcePreviewSummary {
  action: string;
  invalid_count: number;
  mode: SourceIPMode;
  name: string;
  total_count: number;
}

export interface SourcePreviewPayload {
  port_summary?: Record<string, unknown> | null;
  preview_entries: string[];
  source_status: Partial<SourceConfig> | null;
  summary: SourcePreviewSummary | null;
}

export interface ColoDictionaryStatus {
  colo_ipv4_path: string;
  colo_ipv4_rows: number;
  colo_ipv6_path: string;
  colo_ipv6_rows: number;
  colo_path: string;
  colo_rows: number;
  geofeed_path: string;
  geofeed_rows: number;
  last_updated_at: string;
  matched_rows: number;
  missing_rows: number;
  source_url: string;
  updated: boolean;
  unmatched_rows: number;
}
