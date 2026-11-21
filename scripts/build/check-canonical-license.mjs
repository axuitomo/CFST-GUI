// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

// LICENSE 必须是 https://www.gnu.org/licenses/gpl-3.0.txt 的逐字原文（35,149 字节，LF）。
// GitHub 等许可证识别按整段文本比对：在文件头加项目名、版权行、SPDX 标识或注释，
// 把第三方说明写进来，或者把别的协议合并进同一个文件，都会让识别结果变成“无法识别”。
// 项目版权、SPDX 标识和第三方归属写在 README.md 与 THIRD-PARTY-NOTICES.md，不写进 LICENSE。
export const CANONICAL_LICENSE_SHA256 = "3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986";
export const CANONICAL_LICENSE_BYTES = 35149;
const FIRST_LINE = "                    GNU GENERAL PUBLIC LICENSE";
const LAST_LINE = "<https://www.gnu.org/licenses/why-not-lgpl.html>.";

/** 返回 LICENSE 偏离原文的问题列表；空数组表示仍是 GPL-3.0 逐字原文。 */
export function canonicalLicenseIssues(text) {
  const normalized = text.replace(/\r\n/g, "\n");
  const issues = [];

  const bytes = Buffer.byteLength(normalized, "utf8");
  if (bytes !== CANONICAL_LICENSE_BYTES) {
    issues.push(`文件为 ${bytes} 字节，原文为 ${CANONICAL_LICENSE_BYTES} 字节`);
  }

  const digest = createHash("sha256").update(normalized).digest("hex");
  if (digest !== CANONICAL_LICENSE_SHA256) {
    issues.push(`内容哈希 ${digest} 与 gnu.org 原文不一致`);
  }

  if (!normalized.startsWith(`${FIRST_LINE}\n`)) {
    issues.push("开头不是 GPL-3.0 标题行，疑似添加了项目名、版权行、SPDX 标识或注释抬头");
  }
  if (!normalized.endsWith(`${LAST_LINE}\n`)) {
    issues.push("结尾不是原文收尾行，疑似追加了版权附加文字、第三方说明或另一份协议");
  }
  if (/(?:SPDX-License-Identifier|Copyright \(C\) 2026|THIRD-PARTY|第三方)/.test(normalized)) {
    issues.push("正文含项目版权、SPDX 标识或第三方声明等附加文字");
  }
  if (normalized.split(FIRST_LINE).length > 2) {
    issues.push("标题行出现多次，疑似合并了多份协议");
  }

  return issues;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const licensePath = path.join(root, "LICENSE");
  const issues = canonicalLicenseIssues(readFileSync(licensePath, "utf8"));
  if (issues.length > 0) {
    console.error(`LICENSE 校验失败：${licensePath}`);
    for (const issue of issues) console.error(`- ${issue}`);
    console.error(
      "LICENSE 只能存放 GPL-3.0 原文；项目版权、SPDX 标识与第三方声明请写入 README.md 或 THIRD-PARTY-NOTICES.md。",
    );
    process.exitCode = 1;
  } else {
    console.log(`LICENSE 与 GPL-3.0 原文一致（${CANONICAL_LICENSE_BYTES} 字节）。`);
  }
}
