import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const rootDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const agpVersion = process.env.CFST_ANDROID_GRADLE_PLUGIN_VERSION || "9.3.0";
const files = [
  "mobile/android/app/capacitor.build.gradle",
  "mobile/android/capacitor-cordova-android-plugins/build.gradle",
  "frontend/node_modules/@capacitor/android/capacitor/build.gradle",
];

function patchGradle(source) {
  const eol = source.includes("\r\n") ? "\r\n" : "\n";
  let patched = source
    .replace(
      /classpath 'com\.android\.tools\.build:gradle:[^']+'/g,
      `classpath 'com.android.tools.build:gradle:${agpVersion}'`,
    )
    .replace(/=\s+=\s*/g, "= ")
    .replace(
      /System\.getenv\("CAP_PUBLISH"\)\s*=\s*"true"/g,
      'System.getenv("CAP_PUBLISH") == "true"',
    )
    .replace(
      /\r?\n\s*flatDir\s*\{\s*\r?\n\s*dirs [^\r\n]+\r?\n\s*\}\s*\r?\n/g,
      eol,
    );

  const assignments = [
    [/^(\s*)url\s+"([^"]+)"\s*$/gm, '$1url = uri("$2")'],
    [/^(\s*)namespace\s+(?!=)"([^"]+)"\s*$/gm, '$1namespace = "$2"'],
    [/^(\s*)compileSdk\s+(?!=)([^\r\n]+)$/gm, "$1compileSdk = $2"],
    [/^(\s*)minSdkVersion\s+(?!=)([^\r\n]+)$/gm, "$1minSdk = $2"],
    [/^(\s*)targetSdkVersion\s+(?!=)([^\r\n]+)$/gm, "$1targetSdk = $2"],
    [/^(\s*)versionCode\s+(?!=)([^\r\n]+)$/gm, "$1versionCode = $2"],
    [/^(\s*)versionName\s+(?!=)([^\r\n]+)$/gm, "$1versionName = $2"],
    [/^(\s*)minifyEnabled\s+(?!=)([^\r\n]+)$/gm, "$1minifyEnabled = $2"],
    [/^(\s*)abortOnError\s+(?!=)([^\r\n]+)$/gm, "$1abortOnError = $2"],
    [/^(\s*)warningsAsErrors\s+(?!=)([^\r\n]+)$/gm, "$1warningsAsErrors = $2"],
    [/^(\s*)baseline\s+file\(/gm, "$1baseline = file("],
    [/^(\s*)lintConfig\s+file\(/gm, "$1lintConfig = file("],
    [
      /^(\s*)sourceCompatibility\s+(?!=)([^\r\n]+)$/gm,
      "$1sourceCompatibility = $2",
    ],
    [
      /^(\s*)targetCompatibility\s+(?!=)([^\r\n]+)$/gm,
      "$1targetCompatibility = $2",
    ],
    [/^(\s*)lintOptions\s*\{/gm, "$1lint {"],
  ];
  for (const [pattern, replacement] of assignments) {
    patched = patched.replace(pattern, replacement);
  }
  return patched;
}

for (const relativeFile of files) {
  const file = path.join(rootDir, relativeFile);
  let source;
  try {
    source = await readFile(file, "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") continue;
    throw error;
  }
  const patched = patchGradle(source);
  if (patched !== source) {
    await writeFile(file, patched, "utf8");
  }
}
