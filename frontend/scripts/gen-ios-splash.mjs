/**
 * gen-ios-splash.mjs
 *
 * Generates apple-touch-startup-image PNG splash screens for iOS PWA support.
 *
 * Run once manually from the repo root:
 *   node frontend/scripts/gen-ios-splash.mjs
 *
 * The output PNGs are committed to frontend/public/splash/.
 * This script is NOT wired into the prebuild or any CI step.
 *
 * Requirements:
 *   - rsvg-convert (librsvg) at /opt/homebrew/bin/rsvg-convert
 *   - Node.js built-ins only (no npm dependencies)
 */

import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, unlinkSync, writeFileSync } from "node:fs";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const RSVG_CONVERT = "/opt/homebrew/bin/rsvg-convert";

// Physical pixel dimensions for each target device (portrait orientation only).
// Output file name per device: splash-<physW>x<physH>.png
const DEVICES = [
  { w: 1320, h: 2868 }, // iPhone 16 Pro Max
  { w: 1206, h: 2622 }, // iPhone 16 Pro
  { w: 1290, h: 2796 }, // iPhone 16 Plus / 15 Plus
  { w: 1179, h: 2556 }, // iPhone 16 / 15 / 14 Pro
  { w: 1170, h: 2532 }, // iPhone 14 / 13 / 12
  { w: 1284, h: 2778 }, // iPhone 14 Plus / 13 Pro Max
  { w: 1242, h: 2688 }, // iPhone 11 Pro Max / XS Max
  { w: 828, h: 1792 }, // iPhone 11 / XR
  { w: 750, h: 1334 }, // iPhone SE 3rd gen
  { w: 2048, h: 2732 }, // iPad Pro 12.9"
  { w: 1668, h: 2388 }, // iPad Pro 11" / Air 4-5
  { w: 1668, h: 2224 }, // iPad Air 3 / Pro 10.5"
  { w: 1488, h: 2266 }, // iPad mini 6
  { w: 1620, h: 2160 }, // iPad 9th/10th gen
];

// Colors from the flamingo brand palette.
const CORAL = "#FF6F79";

// The flamingo logo paths extracted from frontend/src/app/icon.svg.
// The source SVG has viewBox="0 0 1024 1024". We skip the rounded-rect background
// (rx/ry="153") so the logo renders cleanly against the full-bleed coral splash background.
// Path 1: main flamingo body silhouette in #FDFAF6 (cream white), origin at translate(344,155).
// Path 2: decorative wing accent in #FF6F79, origin at translate(334,482).
// Path 3: decorative neck/beak accent in #FF6F79, origin at translate(351,178).
const FLAMINGO_PATHS_SVG = `
  <g>
    <path
      d="M0,0 L95,0 L103,6 L122,25 L122,27 L124,27 L131,35 L138,41 L145,49 L156,60 L158,64 L158,140 L154,146 L118,182 L111,190 L33,268 L31,268 L30,271 L29,303 L277,302 L288,286 L302,267 L309,257 L323,238 L335,221 L348,203 L362,184 L374,167 L387,149 L393,141 L398,138 L405,138 L411,142 L413,146 L413,153 L407,163 L397,176 L388,189 L375,207 L363,224 L349,243 L336,261 L322,281 L308,300 L307,302 L406,303 L412,307 L414,311 L414,319 L407,327 L390,344 L382,351 L365,368 L357,375 L342,390 L334,397 L313,418 L305,425 L292,438 L290,438 L290,440 L282,447 L265,464 L257,471 L240,488 L232,495 L214,513 L206,520 L193,533 L192,713 L283,713 L289,718 L290,720 L290,730 L285,736 L282,737 L78,737 L72,733 L70,730 L70,720 L75,714 L78,713 L168,713 L167,533 L151,517 L143,510 L120,487 L112,480 L99,467 L91,460 L71,440 L63,433 L47,417 L42,413 L37,408 L18,389 L10,382 L-7,365 L-15,358 L-27,346 L-35,339 L-53,321 L-54,319 L-54,240 L-51,235 L-31,215 L-26,210 L-16,200 L-9,192 L-4,187 L-1,186 L-1,184 L1,184 L3,180 L9,175 L16,167 L23,160 L28,155 L64,119 L65,94 L11,95 L3,102 L-4,110 L-12,117 L-14,120 L-15,167 L-18,172 L-21,175 L-30,176 L-37,172 L-39,168 L-43,166 L-76,133 L-78,129 L-78,77 L-74,71 L-69,66 L-67,66 L-67,64 L-65,64 L-65,62 L-63,62 L-61,58 L-51,48 L-43,41 L-36,34 L-29,26 L-14,12 L-3,1 Z"
      fill="#FDFAF6"
      transform="translate(344,155)"
    />
    <path
      d="M0,0 L268,0 L266,5 L254,21 L242,38 L229,56 L216,74 L203,92 L190,110 L180,124 L179,132 L182,138 L186,140 L194,140 L199,137 L210,121 L223,103 L236,85 L249,67 L263,48 L277,28 L291,9 L298,0 L381,0 L375,7 L363,19 L355,26 L338,43 L330,50 L314,66 L306,73 L289,90 L281,97 L263,115 L261,115 L259,119 L251,126 L240,137 L234,142 L229,147 L211,165 L203,172 L191,184 L187,182 L171,166 L163,159 L142,138 L134,131 L117,114 L109,107 L90,88 L82,81 L65,64 L57,57 L41,41 L33,34 L16,17 L8,10 L0,2 Z"
      fill="#FF6F79"
      transform="translate(334,482)"
    />
    <path
      d="M0,0 L80,0 L90,10 L95,15 L112,32 L119,40 L128,49 L128,109 L107,130 L102,135 L91,147 L85,152 L84,154 L82,154 L82,156 L77,160 L72,166 L70,166 L68,170 L56,182 L54,182 L54,184 L52,184 L52,186 L50,186 L50,188 L45,192 L38,200 L26,212 L19,218 L12,226 L7,230 L6,232 L4,232 L2,236 L0,239 L-1,280 L-38,280 L-38,225 L-12,199 L-7,194 L4,183 L9,178 L20,167 L27,159 L79,107 L80,104 L80,56 L78,51 L73,49 L-5,49 L-12,54 L-18,61 L-26,68 L-31,73 L-38,81 L-43,86 L-44,88 L-45,113 L-52,107 L-60,100 L-62,97 L-62,62 Z"
      fill="#FF6F79"
      transform="translate(351,178)"
    />
  </g>
`;

// The flamingo paths above are drawn in a 1024x1024 coordinate space.
// Bounding box of the combined mark (estimated from path coords + transforms):
//   x: 344 + (-78) = 266  to  344 + 414 = 758   => width ~492
//   y: 155 + 0    = 155   to  155 + 737 = 892    => height ~737
// We use the full 1024x1024 viewBox so scale() works predictably.
const LOGO_VIEWBOX_SIZE = 1024;

/**
 * Build a wrapper SVG string for the given physical dimensions.
 * The flamingo logo is scaled so its height is ~40% of min(w, h), centered.
 *
 * @param {number} w - physical pixel width
 * @param {number} h - physical pixel height
 * @returns {string} SVG markup
 */
function buildSplashSvg(w, h) {
  const shorter = Math.min(w, h);
  // Target logo size in output pixels (the logo occupies roughly 737/1024 of the viewBox height).
  const targetLogoHeight = shorter * 0.4;
  // Scale factor: map 1024 source units to targetLogoHeight output pixels,
  // accounting for the fact that the logo mark is ~737/1024 tall in the source.
  const logoMarkHeightFraction = 737 / LOGO_VIEWBOX_SIZE;
  const scale = targetLogoHeight / (LOGO_VIEWBOX_SIZE * logoMarkHeightFraction);
  // Scaled logo dimensions in output pixels.
  const scaledW = LOGO_VIEWBOX_SIZE * scale;
  const scaledH = LOGO_VIEWBOX_SIZE * scale;
  // Center the logo in the splash.
  const tx = (w - scaledW) / 2;
  const ty = (h - scaledH) / 2;

  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
  <!-- Full-bleed coral background -->
  <rect x="0" y="0" width="${w}" height="${h}" fill="${CORAL}"/>
  <!-- Flamingo logo centered, scaled to ~40% of the shorter dimension -->
  <g transform="translate(${tx.toFixed(2)},${ty.toFixed(2)}) scale(${scale.toFixed(6)})">
    ${FLAMINGO_PATHS_SVG}
  </g>
</svg>
`;
}

function main() {
  const __filename = fileURLToPath(import.meta.url);
  const repoRoot = join(dirname(__filename), "..", "..");
  const splashDir = join(repoRoot, "frontend", "public", "splash");

  if (!existsSync(splashDir)) {
    mkdirSync(splashDir, { recursive: true });
    console.log(`Created directory: ${splashDir}`);
  }

  const tmpDir = os.tmpdir();
  let successCount = 0;
  let failCount = 0;

  for (const { w, h } of DEVICES) {
    const pngName = `splash-${w}x${h}.png`;
    const pngPath = join(splashDir, pngName);
    const tmpSvg = join(tmpDir, `splash-${w}x${h}-tmp.svg`);

    try {
      const svgContent = buildSplashSvg(w, h);
      writeFileSync(tmpSvg, svgContent, "utf8");

      execFileSync(RSVG_CONVERT, ["-w", String(w), "-h", String(h), tmpSvg, "-o", pngPath]);

      successCount++;
      console.log(`  OK  ${pngName}`);
    } catch (err) {
      failCount++;
      console.error(`  FAIL ${pngName}: ${err.message}`);
    } finally {
      if (existsSync(tmpSvg)) {
        unlinkSync(tmpSvg);
      }
    }
  }

  console.log(`\nDone: ${successCount} generated, ${failCount} failed.`);
  console.log(`Output: ${splashDir}`);
  if (failCount > 0) process.exitCode = 1;
}

main();
