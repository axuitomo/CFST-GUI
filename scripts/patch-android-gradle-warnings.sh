#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGP_VERSION="${CFST_ANDROID_GRADLE_PLUGIN_VERSION:-9.2.0}"

patch_gradle_file() {
  local file="$1"
  [[ -f "$file" ]] || return 0

  perl -0pi -e "s/classpath 'com\\.android\\.tools\\.build:gradle:[^']+'/classpath 'com.android.tools.build:gradle:$AGP_VERSION'/g" "$file"
  perl -0pi -e 's/=\s+=\s*/= /g' "$file"
  perl -0pi -e 's/System\.getenv\("CAP_PUBLISH"\)\s=\s"true"/System.getenv("CAP_PUBLISH") == "true"/g' "$file"
  perl -0pi -e 's/\n\s*flatDir\s*\{\s*\n\s*dirs [^\n]+\n\s*\}\s*\n/\n/g' "$file"

  perl -0pi -e 's/^(\s*)url\s+(?!="?=)"([^"]+)"\s*$/$1url = uri("$2")/mg' "$file"
  perl -0pi -e 's/^(\s*)namespace\s+(?!=)"([^"]+)"\s*$/$1namespace = "$2"/mg' "$file"
  perl -0pi -e 's/^(\s*)compileSdk\s+(?!=)(.+)$/$1compileSdk = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)minSdkVersion\s+(?!=)(.+)$/$1minSdk = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)targetSdkVersion\s+(?!=)(.+)$/$1targetSdk = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)versionCode\s+(?!=)(.+)$/$1versionCode = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)versionName\s+(?!=)(.+)$/$1versionName = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)minifyEnabled\s+(?!=)(.+)$/$1minifyEnabled = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)abortOnError\s+(?!=)(.+)$/$1abortOnError = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)warningsAsErrors\s+(?!=)(.+)$/$1warningsAsErrors = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)baseline\s+file\(/$1baseline = file(/mg' "$file"
  perl -0pi -e 's/^(\s*)lintConfig\s+file\(/$1lintConfig = file(/mg' "$file"
  perl -0pi -e 's/^(\s*)sourceCompatibility\s+(?!=)(.+)$/$1sourceCompatibility = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)targetCompatibility\s+(?!=)(.+)$/$1targetCompatibility = $2/mg' "$file"
  perl -0pi -e 's/^(\s*)lintOptions\s*\{/$1lint {/mg' "$file"
}

patch_gradle_file "$ROOT_DIR/mobile/android/app/capacitor.build.gradle"
patch_gradle_file "$ROOT_DIR/mobile/android/capacitor-cordova-android-plugins/build.gradle"
patch_gradle_file "$ROOT_DIR/frontend/node_modules/@capacitor/android/capacitor/build.gradle"
