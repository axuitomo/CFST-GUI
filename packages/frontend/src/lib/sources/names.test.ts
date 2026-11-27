import { describe, expect, it } from "vitest";
import { detectSourceNameFromUrl } from "./names";

// 链接与期望名称都取自 https://bestcf.pages.dev/ 的「优选IP BestIP」「优选网站」清单。
const CASES: [string, string][] = [
  ["https://bestcf.pages.dev/wetest/ipv4.txt", "WeTest"],
  ["https://www.wetest.vip/page/cloudfront/address_v4.html", "WeTest"],
  ["https://api.uouin.com/cloudflare.html", "UOUIN"],
  ["https://bestcf.pages.dev/uouin/all.txt", "UOUIN"],
  ["https://bestcf.pages.dev/xinyitang3/ipv4.txt", "Mia"],
  ["https://bestcf.pages.dev/lzj/all.txt", "LZJ"],
  ["https://bestcf.pages.dev/cfyes/ipv6.txt", "CFYes"],
  ["https://stock.hostmonit.com/CloudFlareYes", "HostMonit"],
  ["https://api.soso.sryze.cc/AutoSubFBot.txt", "天诚 5"],
  ["https://raw.githubusercontent.com/gshtwy/CF-DNS-Clone/refs/heads/main/wetest-cloudflare-v4.txt", "天诚 4"],
  ["https://bestcf.pages.dev/tiancheng/hk.txt", "天诚 1"],
  ["https://bestcf.pages.dev/tiancheng2/de.txt", "天诚 2"],
  ["https://bestcf.pages.dev/tiancheng3/all.txt", "天诚 3"],
  ["https://bestcf.pages.dev/gslege/SG.txt", "Gslege"],
  ["https://cf.junzhen.qzz.io/best_ips.txt", "MJZ"],
  ["https://raw.githubusercontent.com/svip-s/cloudflare_ip/refs/heads/main/best_ips.txt", "SS"],
  ["https://raw.githubusercontent.com/love-ztm/cfip/refs/heads/main/ubest_ips.txt", "LZ"],
  ["https://bestcf.pages.dev/zhixuanwang/ipv4-onlyip.txt", "ZhiXuan"],
  ["https://bestcf.pages.dev/vvhan/ipv6.txt", "vvHan"],
  ["https://cf.vvhan.com/", "vvHan"],
  ["https://raw.githubusercontent.com/ymyuuu/IPDB/refs/heads/main/BestCF/bestcfv4.txt", "MingYu"],
  ["https://raw.githubusercontent.com/yuanxiawan/cfipv4db/refs/heads/main/cfip.txt", "Yuanxiawan"],
  ["https://bestcf.pages.dev/cmliu/all.txt", "CM 1"],
  ["https://raw.githubusercontent.com/cmliu/WorkerVless2sub/refs/heads/main/addressesapi.txt", "CM 3"],
  ["https://addressesapi.090227.xyz/CloudFlareYes", "CM API"],
  ["https://cf.090227.xyz/", "CM"],
  ["https://090227.pages.dev/bestcf?isp=ct&ips=50", "CM"],
  ["https://raw.githubusercontent.com/LancelotRar/best-cf-ips/refs/heads/main/best-cf-ip-scanned-top100.txt", "LancelotRar"],
  ["https://raw.githubusercontent.com/joname1/BestCFip/refs/heads/main/ipv6.txt", "Joname"],
  ["https://raw.githubusercontent.com/ahang39/router/refs/heads/main/all.txt", "Ahang"],
  ["https://raw.githubusercontent.com/einsitang/my-fast-cf-ip/refs/heads/master/fastips.txt", "Einsitang"],
  ["https://raw.githubusercontent.com/hubbylei/bestcf/refs/heads/main/bestcf.txt", "Hubbylei"],
  ["https://raw.githubusercontent.com/Senflare/Senflare-IP/refs/heads/main/IPlist-Pro.txt", "Senflare"],
  ["https://raw.githubusercontent.com/JieChaoCC/cf-ip-auto/refs/heads/main/data/ipapi.txt", "JieChaoCC"],
  ["https://bestcf.pages.dev/ircf/ipv4.txt", "IRCF"],
  ["https://bestcf.pages.dev/nirevil/ipv6.txt", "NiREvil"],
  ["https://bestcf.pages.dev/entryip/50.txt", "Endpoint IP"],
  ["https://bestcf.pages.dev/luoli/all.txt", "LuoLi"],
  ["https://bestcf.pages.dev/kristi/all.txt", "Kristi"],
  ["https://bestcf.pages.dev/moistr/all.txt", "MoistR"],
  ["https://bestcf.pages.dev/yutian/all.txt", "YuTian"],
  ["https://raw.githubusercontent.com/Fiatnorm/OptiDomain-Pages/refs/heads/main/optimized_cf_ips.txt", "Fiatnorm"],
  ["https://vps789.com/cfip/?remarks=domain", "VPS789"],
  ["https://ipdb.api.030101.xyz/?type=cfv4", "IPDB"],
  ["https://www.byoip.top/", "NB"],
  ["https://github.com/HandsomeMJZ/cfip-tools", "HandsomeMJZ"],
];

describe("source name detection", () => {
  it("names the sources listed on bestcf.pages.dev", () => {
    for (const [url, name] of CASES) {
      expect(detectSourceNameFromUrl(url), url).toBe(name);
    }
  });

  it("falls back to the provider segment instead of the file format", () => {
    expect(detectSourceNameFromUrl("https://bestcf.pages.dev/hanfeng/ipv4.txt")).toBe("Hanfeng");
    expect(detectSourceNameFromUrl("https://unknown.example.com/best-cf-ips/all.txt")).toBe("Best Cf Ips");
  });

  it("keeps the default marker for auto-fill decisions", () => {
    expect(detectSourceNameFromUrl("")).toBe("");
  });
});
