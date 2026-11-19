# Third-Party Notices

CFST-GUI is licensed under GPL-3.0-only. This notice covers frontend production npm packages, bundled Tailwind CSS, Capacitor Android, resolved Go package graphs for release targets, Android releaseRuntimeClasspath, and the Go standard library/runtime bundled in Go executables. OS-provided libraries are excluded.

Android graph regeneration (from `mobile/android`): `./gradlew -I ../../scripts/build/android-license-report.init.gradle --no-configuration-cache :app:generateLicenseReport`, then `node scripts/build/check-android-license-report.mjs --update`; regenerate the combined notice with `node scripts/build/generate-third-party-notices.mjs --write`. Android release/debug builds rerun the Gradle report and check the snapshot.

## Frontend and Capacitor Packages

| Package | Version | License | Scope | Author metadata |
| --- | --- | --- | --- | --- |
| @capacitor/android | 8.6.2 | MIT | Android runtime | Ionic Team <hi@ionic.io> (<https://ionic.io>) |
| tailwindcss | 4.3.5 | MIT | Bundled CSS | Not specified |
| @babel/helper-string-parser | 7.29.14 | MIT | Frontend runtime | The Babel Team (<https://babel.dev/team>) |
| @babel/helper-validator-identifier | 7.29.7 | MIT | Frontend runtime | The Babel Team (<https://babel.dev/team>) |
| @babel/parser | 7.29.14 | MIT | Frontend runtime | The Babel Team (<https://babel.dev/team>) |
| @babel/types | 7.29.14 | MIT | Frontend runtime | The Babel Team (<https://babel.dev/team>) |
| @capacitor/core | 8.6.2 | MIT | Frontend runtime | Ionic Team <hi@ionic.io> (<https://ionic.io>) |
| @jridgewell/sourcemap-codec | 1.6.0 | MIT | Frontend runtime | Justin Ridgewell <justin@ridgewell.name> |
| @phosphor-icons/vue | 2.2.1 | MIT | Frontend runtime | Not specified |
| @types/web-bluetooth | 0.0.21 | MIT | Frontend runtime | Not specified |
| @vue/compiler-core | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/compiler-dom | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/compiler-sfc | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/compiler-ssr | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/compiler-vapor | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/reactivity | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/runtime-core | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/runtime-dom | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/runtime-vapor | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/server-renderer | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vue/shared | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |
| @vueuse/core | 15.0.0 | MIT | Frontend runtime | Anthony Fu <<https://github.com/antfu>> |
| @vueuse/metadata | 15.0.0 | MIT | Frontend runtime | Anthony Fu <<https://github.com/antfu>> |
| @vueuse/shared | 15.0.0 | MIT | Frontend runtime | Anthony Fu <<https://github.com/antfu>> |
| @wailsio/runtime | 3.2.0-nightly.20261103 | MIT | Frontend runtime | The Wails Team |
| csstype | 3.2.3 | MIT | Frontend runtime | Fredrik Nicol <fredrik.nicol@gmail.com> |
| entities | 7.0.1 | BSD-2-Clause | Frontend runtime | Felix Boehm <me@feedic.com> |
| estree-walker | 2.0.2 | MIT | Frontend runtime | Rich Harris |
| framer-motion | 14.14.0 | MIT | Frontend runtime | Matt Perry |
| hey-listen | 1.0.8 | MIT | Frontend runtime | Matt Perry |
| magic-string | 0.30.21 | MIT | Frontend runtime | Rich Harris |
| motion-dom | 14.14.0 | MIT | Frontend runtime | Matt Perry |
| motion-utils | 14.7.0 | MIT | Frontend runtime | Matt Perry |
| motion-v | 2.13.0 | MIT | Frontend runtime | Not specified |
| nanoid | 3.3.20 | MIT | Frontend runtime | Andrey Sitnik <andrey@sitnik.ru> |
| picocolors | 1.1.1 | ISC | Frontend runtime | Alexey Raspopov |
| postcss | 8.5.32 | MIT | Frontend runtime | Andrey Sitnik <andrey@sitnik.es> |
| source-map-js | 1.2.2 | BSD-3-Clause | Frontend runtime | Valentin 7rulnik Semirulnik <v7rulnik@gmail.com> |
| tslib | 2.8.1 | 0BSD | Frontend runtime | Microsoft Corp. |
| vue | 3.6.0-rc.15 | MIT | Frontend runtime | Evan You |

## Android Release Runtime (Maven)

Coordinates and declared licenses below come from resolved `releaseRuntimeClasspath` POM metadata. Legal texts found inside the resolved AAR/JAR archives (including nested JARs and local libraries) are also reproduced below.

| Module | Version | License | POM project URL |
| --- | --- | --- | --- |
| androidx.activity:activity | 1.13.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/activity#1.13.0> |
| androidx.annotation:annotation | 1.10.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/annotation#1.10.0> |
| androidx.annotation:annotation-experimental | 1.4.1 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/annotation#1.4.1> |
| androidx.annotation:annotation-jvm | 1.10.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/annotation#1.10.0> |
| androidx.appcompat:appcompat | 1.8.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/appcompat#1.8.0> |
| androidx.appcompat:appcompat-resources | 1.8.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/appcompat#1.8.0> |
| androidx.arch.core:core-common | 2.2.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/arch-core#2.2.0> |
| androidx.arch.core:core-runtime | 2.2.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/arch-core#2.2.0> |
| androidx.collection:collection | 1.4.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/collection#1.4.2> |
| androidx.collection:collection-jvm | 1.4.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/collection#1.4.2> |
| androidx.compose.runtime:runtime-annotation | 1.9.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/compose-runtime#1.9.0> |
| androidx.compose.runtime:runtime-annotation-android | 1.9.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/compose-runtime#1.9.0> |
| androidx.concurrent:concurrent-futures | 1.1.0 | Apache-2.0 | <https://developer.android.com/topic/libraries/architecture/index.html> |
| androidx.concurrent:concurrent-futures-ktx | 1.1.0 | Apache-2.0 | <https://developer.android.com/topic/libraries/architecture/index.html> |
| androidx.coordinatorlayout:coordinatorlayout | 1.3.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/coordinatorlayout#1.3.0> |
| androidx.core:core | 1.19.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/core#1.19.0> |
| androidx.core:core-ktx | 1.19.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/core#1.19.0> |
| androidx.core:core-splashscreen | 1.2.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/core#1.2.0> |
| androidx.core:core-viewtree | 1.0.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/core#1.0.0> |
| androidx.cursoradapter:cursoradapter | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.customview:customview | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.documentfile:documentfile | 1.1.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/documentfile#1.1.0> |
| androidx.drawerlayout:drawerlayout | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.emoji2:emoji2 | 1.3.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/emoji2#1.3.0> |
| androidx.emoji2:emoji2-views-helper | 1.3.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/emoji2#1.3.0> |
| androidx.fragment:fragment | 1.8.9 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/fragment#1.8.9> |
| androidx.interpolator:interpolator | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.lifecycle:lifecycle-common | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-livedata | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-livedata-core | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-process | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-runtime | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-service | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-viewmodel | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.lifecycle:lifecycle-viewmodel-savedstate | 2.6.2 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/lifecycle#2.6.2> |
| androidx.loader:loader | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.navigationevent:navigationevent | 1.0.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/navigationevent#1.0.0> |
| androidx.navigationevent:navigationevent-android | 1.0.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/navigationevent#1.0.0> |
| androidx.profileinstaller:profileinstaller | 1.4.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/profileinstaller#1.4.0> |
| androidx.resourceinspection:resourceinspection-annotation | 1.0.1 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/resourceinspection#1.0.1> |
| androidx.room:room-common | 2.7.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/room#2.7.0> |
| androidx.room:room-common-jvm | 2.7.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/room#2.7.0> |
| androidx.room:room-runtime | 2.7.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/room#2.7.0> |
| androidx.room:room-runtime-android | 2.7.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/room#2.7.0> |
| androidx.savedstate:savedstate | 1.2.1 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/savedstate#1.2.1> |
| androidx.sqlite:sqlite | 2.5.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/sqlite#2.5.0> |
| androidx.sqlite:sqlite-android | 2.5.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/sqlite#2.5.0> |
| androidx.sqlite:sqlite-framework | 2.5.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/sqlite#2.5.0> |
| androidx.sqlite:sqlite-framework-android | 2.5.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/sqlite#2.5.0> |
| androidx.startup:startup-runtime | 1.1.1 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/startup#1.1.1> |
| androidx.tracing:tracing | 1.2.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/tracing#1.2.0> |
| androidx.tracing:tracing-ktx | 1.2.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/tracing#1.2.0> |
| androidx.vectordrawable:vectordrawable | 1.1.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx> |
| androidx.vectordrawable:vectordrawable-animated | 1.1.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx> |
| androidx.versionedparcelable:versionedparcelable | 1.1.1 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.viewpager:viewpager | 1.0.0 | Apache-2.0 | <http://developer.android.com/tools/extras/support-library.html> |
| androidx.webkit:webkit | 1.16.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/webkit#1.16.0> |
| androidx.work:work-runtime | 2.12.0 | Apache-2.0 | <https://developer.android.com/jetpack/androidx/releases/work#2.12.0> |
| com.google.guava:listenablefuture | 1.0 | Apache-2.0 | Not specified |
| org.apache.cordova:framework | 15.0.0 | Apache-2.0 | <https://cordova.apache.org> |
| org.jetbrains.kotlin:kotlin-stdlib | 2.4.21 | Apache-2.0 | <https://kotlinlang.org/> |
| org.jetbrains.kotlinx:kotlinx-coroutines-android | 1.9.0 | Apache-2.0 | <https://github.com/Kotlin/kotlinx.coroutines> |
| org.jetbrains.kotlinx:kotlinx-coroutines-bom | 1.9.0 | Apache-2.0 | <https://github.com/Kotlin/kotlinx.coroutines> |
| org.jetbrains.kotlinx:kotlinx-coroutines-core | 1.9.0 | Apache-2.0 | <https://github.com/Kotlin/kotlinx.coroutines> |
| org.jetbrains.kotlinx:kotlinx-coroutines-core-jvm | 1.9.0 | Apache-2.0 | <https://github.com/Kotlin/kotlinx.coroutines> |
| org.jetbrains:annotations | 23.0.0 | Apache-2.0 | <https://github.com/JetBrains/java-annotations> |
| org.jspecify:jspecify | 1.0.0 | Apache-2.0 | <http://jspecify.org/> |

## Go Toolchain Runtime

go1.27.4 standard library/runtime code is bundled in the Go executables; its BSD license, patent notice, and vendored-source licenses are reproduced in the license-text section below.

## Go Dependencies by Release Target

Go module license files were collected with google/go-licenses for Windows desktop/installer, Linux WebUI, macOS desktop, and the Android gomobile AAR, including both amd64 and arm64 Linux/macOS targets. The generated gomobile binding runtime is attributed from the pinned golang.org/x/mobile module. Supplemental legal files and inline grants from the standard library and selected module source trees are preserved as well; these can include files not linked on every target.

| Module | Version | Targets | License files |
| --- | --- | --- | --- |
| git.sr.ht/~jackmordaunt/go-toast/v2 | v2.0.3 | Windows desktop/installer | git.sr.ht/~jackmordaunt/go-toast/v2/LICENSE |
| github.com/VividCortex/ewma | v1.2.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/VividCortex/ewma/LICENSE |
| github.com/cespare/xxhash/v2 | v2.3.0 | Windows desktop/installer | github.com/cespare/xxhash/v2/LICENSE.txt |
| github.com/cheggaaa/pb/v3 | v3.3.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/cheggaaa/pb/v3/LICENSE |
| github.com/clipperhouse/uax29/v2 | v2.7.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/clipperhouse/uax29/v2/LICENSE, github.com/clipperhouse/uax29/v2/graphemes/LICENSE |
| github.com/fatih/color | v1.19.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/fatih/color/LICENSE.md |
| github.com/getlantern/context | v0.0.0-20220418194847-3d5e7a086201 | Windows desktop/installer | github.com/getlantern/context/LICENSE |
| github.com/getlantern/errors | v1.0.4 | Windows desktop/installer | github.com/getlantern/errors/LICENSE |
| github.com/getlantern/golog | v0.0.0-20230503153817-8e72de7e0a65 | Windows desktop/installer | github.com/getlantern/golog/LICENSE |
| github.com/getlantern/hex | v0.0.0-20220104173244-ad7e4b9194dc | Windows desktop/installer | github.com/getlantern/hex/LICENSE |
| github.com/getlantern/hidden | v0.0.0-20220104173330-f221c5a24770 | Windows desktop/installer | github.com/getlantern/hidden/LICENSE |
| github.com/getlantern/ops | v0.0.0-20231025133620-f368ab734534 | Windows desktop/installer | github.com/getlantern/ops/LICENSE |
| github.com/getlantern/systray | v1.2.2 | Windows desktop/installer | github.com/getlantern/systray/LICENSE |
| github.com/go-logr/logr | v1.4.4 | Windows desktop/installer | github.com/go-logr/logr/LICENSE |
| github.com/go-logr/stdr | v1.2.2 | Windows desktop/installer | github.com/go-logr/stdr/LICENSE |
| github.com/go-ole/go-ole | v1.3.0 | Windows desktop/installer | github.com/go-ole/go-ole/LICENSE |
| github.com/go-stack/stack | v1.8.1 | Windows desktop/installer | github.com/go-stack/stack/LICENSE.md |
| github.com/mattn/go-colorable | v0.1.16 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/mattn/go-colorable/LICENSE |
| github.com/mattn/go-isatty | v0.0.24 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/mattn/go-isatty/LICENSE |
| github.com/mattn/go-runewidth | v0.0.31 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/mattn/go-runewidth/LICENSE |
| github.com/oxtoacart/bpool | v0.0.0-20190530202638-03653db5a59c | Windows desktop/installer | github.com/oxtoacart/bpool/LICENSE |
| github.com/quic-go/qpack | v0.6.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/quic-go/qpack/LICENSE.md |
| github.com/quic-go/quic-go | v0.65.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/quic-go/quic-go/LICENSE, github.com/quic-go/quic-go/assets/LICENSE.md |
| github.com/wailsapp/wails/v3 | v3.2.0-nightly.20261103 | Windows desktop/installer, macOS desktop, macOS desktop arm64 | github.com/wailsapp/wails/v3/LICENSE, github.com/wailsapp/wails/v3/internal/go-common-file-dialog/LICENSE, github.com/wailsapp/wails/v3/internal/go-common-file-dialog/cfd/LICENSE, github.com/wailsapp/wails/v3/internal/templates/ios/frontend/public/puppertino/LICENSE, github.com/wailsapp/wails/v3/internal/webview2/webviewloader/LICENSE |
| go.opentelemetry.io/auto/sdk | v1.2.1 | Windows desktop/installer | go.opentelemetry.io/auto/sdk/LICENSE |
| go.opentelemetry.io/otel | v1.47.0 | Windows desktop/installer | go.opentelemetry.io/otel/LICENSE |
| go.opentelemetry.io/otel/log | v1.47.0 | Windows desktop/installer | go.opentelemetry.io/otel/log/LICENSE |
| go.opentelemetry.io/otel/metric | v1.47.0 | Windows desktop/installer | go.opentelemetry.io/otel/metric/LICENSE |
| go.opentelemetry.io/otel/trace | v1.47.0 | Windows desktop/installer | go.opentelemetry.io/otel/trace/LICENSE |
| go.uber.org/multierr | v1.11.0 | Windows desktop/installer | go.uber.org/multierr/LICENSE.txt |
| go.uber.org/zap | v1.28.0 | Windows desktop/installer | go.uber.org/zap/LICENSE |
| golang.org/x/crypto | v0.59.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | golang.org/x/crypto/LICENSE |
| golang.org/x/mobile | v0.0.0-20261117202452-47699113848a | Android gomobile binding runtime | golang.org/x/mobile/LICENSE |
| golang.org/x/net | v0.63.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | golang.org/x/net/LICENSE |
| golang.org/x/sys | v0.50.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | golang.org/x/sys/LICENSE, golang.org/x/sys/unix/LICENSE |
| golang.org/x/term | v0.48.0 | macOS desktop, macOS desktop arm64 | golang.org/x/term/LICENSE |
| golang.org/x/text | v0.44.0 | Android gomobile AAR, Linux WebUI, Linux WebUI arm64, Windows desktop/installer, macOS desktop, macOS desktop arm64 | golang.org/x/text/LICENSE |

## Upstream Source Acknowledgment

The project README identifies [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) as the basis of the initial core speed-testing logic. The retained upstream history credits xiu2, mz, mazhuang, Spedoske, mikuru996, CrazyBoyFeng, Mac_Zhou, charSLee013, kaka, zhangsean, and other contributors. The upstream `LICENSE` is GNU GPL Version 3; the complete GPLv3 text is included in this distribution's [LICENSE](LICENSE). The source history confirms lineage, but this notice does not claim that every current source file is verbatim upstream code.

## License and Notice Texts

<!-- markdownlint-disable MD010 -->

### MIT

Source files: `@babel/helper-string-parser@7.29.14 (LICENSE)`, `@babel/helper-validator-identifier@7.29.7 (LICENSE)`, `@babel/types@7.29.14 (LICENSE)`

````text
MIT License

Copyright (c) 2014-present Sebastian McKenzie and other contributors

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### MIT

Source files: `@babel/parser@7.29.14 (LICENSE)`

````text
Copyright (C) 2012-2014 by various contributors (see AUTHORS)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
````

### MIT

Source files: `@capacitor/android@8.6.2 (LICENSE)`, `@capacitor/core@8.6.2 (LICENSE)`

````text
MIT License

Copyright (c) 2017-present Drifty Co.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `@jridgewell/sourcemap-codec@1.6.0 (LICENSE)`

````text
Copyright 2024 Justin Ridgewell <justin@ridgewell.name>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `@phosphor-icons/vue@2.2.1 (LICENSE)`

````text
MIT License

Copyright (c) 2020 Phosphor Icons

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `@types/web-bluetooth@0.0.21 (LICENSE)`

````text
    MIT License

    Copyright (c) Microsoft Corporation.

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE
````

### MIT

Source files: `@vue/compiler-core@3.6.0-rc.15 (LICENSE)`, `@vue/compiler-dom@3.6.0-rc.15 (LICENSE)`, `@vue/compiler-sfc@3.6.0-rc.15 (LICENSE)`, `@vue/compiler-ssr@3.6.0-rc.15 (LICENSE)`, `@vue/compiler-vapor@3.6.0-rc.15 (LICENSE)`, `@vue/reactivity@3.6.0-rc.15 (LICENSE)`, `@vue/runtime-core@3.6.0-rc.15 (LICENSE)`, `@vue/runtime-dom@3.6.0-rc.15 (LICENSE)`, `@vue/runtime-vapor@3.6.0-rc.15 (LICENSE)`, `@vue/server-renderer@3.6.0-rc.15 (LICENSE)`, `@vue/shared@3.6.0-rc.15 (LICENSE)`, `vue@3.6.0-rc.15 (LICENSE)`

````text
The MIT License (MIT)

Copyright (c) 2018-present, Yuxi (Evan) You

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
````

### MIT

Source files: `@vueuse/core@15.0.0 (LICENSE)`, `@vueuse/metadata@15.0.0 (LICENSE)`, `@vueuse/shared@15.0.0 (LICENSE)`

````text
MIT License

Copyright (c) 2019-PRESENT Anthony Fu<https://github.com/antfu>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice, MIT

Source files: `@wailsio/runtime@3.2.0-nightly.20261103 (Wails LICENSE)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/LICENSE)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/LICENSE; Windows desktop/installer)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/LICENSE; macOS desktop arm64)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/LICENSE; macOS desktop)`

````text
MIT License

Copyright (c) 2018-Present Lea Anthony

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Android archive license/notice, Apache-2.0

Source files: `Android Release runtime (Apache-2.0)`, `androidx.activity:activity:1.13.0/activity-1.13.0.aar!/META-INF/androidx/activity/activity/LICENSE.txt`, `androidx.annotation:annotation-jvm:1.10.0/annotation-jvm-1.10.0.jar!/META-INF/androidx/annotation/annotation/LICENSE.txt`, `androidx.appcompat:appcompat-resources:1.8.0/appcompat-resources-1.8.0.aar!/META-INF/androidx/appcompat/appcompat-resources/LICENSE.txt`, `androidx.appcompat:appcompat:1.8.0/appcompat-1.8.0.aar!/META-INF/androidx/appcompat/appcompat/LICENSE.txt`, `androidx.compose.runtime:runtime-annotation-android:1.9.0/runtime-annotation.aar!/META-INF/androidx/compose/runtime/runtime-annotation/LICENSE.txt`, `androidx.coordinatorlayout:coordinatorlayout:1.3.0/coordinatorlayout-1.3.0.aar!/META-INF/androidx/coordinatorlayout/coordinatorlayout/LICENSE.txt`, `androidx.core:core-ktx:1.19.0/core-ktx-1.19.0.aar!/META-INF/androidx/core/core-ktx/LICENSE.txt`, `androidx.core:core-splashscreen:1.2.0/core-splashscreen-1.2.0.aar!/META-INF/androidx/core/core-splashscreen/LICENSE.txt`, `androidx.core:core-viewtree:1.0.0/core-viewtree-1.0.0.aar!/META-INF/androidx/core/core-viewtree/LICENSE.txt`, `androidx.core:core:1.19.0/core-1.19.0.aar!/META-INF/androidx/core/core/LICENSE.txt`, `androidx.documentfile:documentfile:1.1.0/documentfile-1.1.0.aar!/META-INF/androidx/documentfile/documentfile/LICENSE.txt`, `androidx.navigationevent:navigationevent-android:1.0.0/navigationevent.aar!/META-INF/androidx/navigationevent/navigationevent/LICENSE.txt`, `androidx.profileinstaller:profileinstaller:1.4.0/profileinstaller-1.4.0.aar!/META-INF/androidx/profileinstaller/profileinstaller/LICENSE.txt`, `androidx.room:room-common-jvm:2.7.0/room-common-jvm-2.7.0.jar!/META-INF/androidx/room/room-common/LICENSE.txt`, `androidx.room:room-runtime-android:2.7.0/room-runtime-release.aar!/META-INF/androidx/room/room-runtime/LICENSE.txt`, `androidx.sqlite:sqlite-android:2.5.0/sqlite-release.aar!/META-INF/androidx/sqlite/sqlite/LICENSE.txt`, `androidx.sqlite:sqlite-framework-android:2.5.0/sqlite-framework-release.aar!/META-INF/androidx/sqlite/sqlite-framework/LICENSE.txt`, `androidx.webkit:webkit:1.16.0/webkit-1.16.0.aar!/META-INF/androidx/webkit/webkit/LICENSE.txt`, `androidx.work:work-runtime:2.12.0/work-runtime-2.12.0.aar!/META-INF/androidx/work/work-runtime/LICENSE.txt`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS
````

### MIT

Source files: `csstype@3.2.3 (LICENSE)`

````text
Copyright (c) 2017-2018 Fredrik Nicol

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### BSD-2-Clause

Source files: `entities@7.0.1 (LICENSE)`

````text
Copyright (c) Felix Böhm
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS,
EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### MIT

Source files: `estree-walker@2.0.2 (LICENSE)`

````text
Copyright (c) 2015-20 [these people](https://github.com/Rich-Harris/estree-walker/graphs/contributors)

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### MIT

Source files: `framer-motion@14.14.0 (LICENSE.md)`

````text
The MIT License (MIT)

Copyright (c) 2018 Framer B.V.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3 (git.sr.ht/~jackmordaunt/go-toast/v2/LICENSE)`, `git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3 (git.sr.ht/~jackmordaunt/go-toast/v2/LICENSE; Windows desktop/installer)`

````text
This project is dual-licensed under the UNLICENSE or
the MIT license with the SPDX identifier:

SPDX-License-Identifier: Unlicense OR MIT

You may use the project under the terms of either license.

Both licenses are reproduced below.

----
The MIT License (MIT)

Copyright (c) 2023 Jack Mordaunt

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
---

---
The UNLICENSE

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

In jurisdictions that recognize copyright laws, the author or authors
of this software dedicate any and all copyright interest in the
software to the public domain. We make this dedication for the benefit
of the public at large and to the detriment of our heirs and
successors. We intend this dedication to be an overt act of
relinquishment in perpetuity of all present and future rights to this
software under copyright law.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
OTHER DEALINGS IN THE SOFTWARE.

For more information, please refer to <https://unlicense.org/>
---
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; Android gomobile AAR)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; Linux WebUI arm64)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; Linux WebUI)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; Windows desktop/installer)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; macOS desktop arm64)`, `github.com/VividCortex/ewma@v1.2.0 (github.com/VividCortex/ewma/LICENSE; macOS desktop)`

````text
The MIT License

Copyright (c) 2013 VividCortex

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/cespare/xxhash/v2@v2.3.0 (github.com/cespare/xxhash/v2/LICENSE.txt)`, `github.com/cespare/xxhash/v2@v2.3.0 (github.com/cespare/xxhash/v2/LICENSE.txt; Windows desktop/installer)`

````text
Copyright (c) 2016 Caleb Spare

MIT License

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; Android gomobile AAR)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; Linux WebUI arm64)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; Linux WebUI)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; Windows desktop/installer)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; macOS desktop arm64)`, `github.com/cheggaaa/pb/v3@v3.3.0 (github.com/cheggaaa/pb/v3/LICENSE; macOS desktop)`

````text
Copyright (c) 2012-2024, Sergey Cherepanov
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

* Neither the name of the author nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/LICENSE)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; Android gomobile AAR)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; Linux WebUI arm64)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; Linux WebUI)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; Windows desktop/installer)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; macOS desktop arm64)`, `github.com/clipperhouse/uax29/v2@v2.7.0 (github.com/clipperhouse/uax29/v2/graphemes/LICENSE; macOS desktop)`

````text
MIT License

Copyright (c) 2020 Matt Sherman

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; Android gomobile AAR)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; Linux WebUI arm64)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; Linux WebUI)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; Windows desktop/installer)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; macOS desktop arm64)`, `github.com/fatih/color@v1.19.0 (github.com/fatih/color/LICENSE.md; macOS desktop)`

````text
The MIT License (MIT)

Copyright (c) 2013 Fatih Arslan

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/getlantern/context@v0.0.0-20220418194847-3d5e7a086201 (github.com/getlantern/context/LICENSE)`, `github.com/getlantern/context@v0.0.0-20220418194847-3d5e7a086201 (github.com/getlantern/context/LICENSE; Windows desktop/installer)`, `github.com/getlantern/hidden@v0.0.0-20220104173330-f221c5a24770 (github.com/getlantern/hidden/LICENSE)`, `github.com/getlantern/hidden@v0.0.0-20220104173330-f221c5a24770 (github.com/getlantern/hidden/LICENSE; Windows desktop/installer)`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright 2016 Brave New Software Project, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/getlantern/errors@v1.0.4 (github.com/getlantern/errors/LICENSE)`, `github.com/getlantern/errors@v1.0.4 (github.com/getlantern/errors/LICENSE; Windows desktop/installer)`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright 2021 Brave New Software Project, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/getlantern/golog@v0.0.0-20230503153817-8e72de7e0a65 (github.com/getlantern/golog/LICENSE)`, `github.com/getlantern/golog@v0.0.0-20230503153817-8e72de7e0a65 (github.com/getlantern/golog/LICENSE; Windows desktop/installer)`, `github.com/getlantern/systray@v1.2.2 (github.com/getlantern/systray/LICENSE)`, `github.com/getlantern/systray@v1.2.2 (github.com/getlantern/systray/LICENSE; Windows desktop/installer)`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright 2014 Brave New Software Project, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/getlantern/hex@v0.0.0-20220104173244-ad7e4b9194dc (github.com/getlantern/hex/LICENSE)`, `github.com/getlantern/hex@v0.0.0-20220104173244-ad7e4b9194dc (github.com/getlantern/hex/LICENSE; Windows desktop/installer)`

````text
Copyright (c) 2009 The Go Authors.
Copyright (c) 2016 Brave New Software Project, Inc.
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/getlantern/ops@v0.0.0-20231025133620-f368ab734534 (github.com/getlantern/ops/LICENSE)`, `github.com/getlantern/ops@v0.0.0-20231025133620-f368ab734534 (github.com/getlantern/ops/LICENSE; Windows desktop/installer)`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright 2017 Brave New Software Project, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/go-logr/logr@v1.4.4 (github.com/go-logr/logr/LICENSE)`, `github.com/go-logr/logr@v1.4.4 (github.com/go-logr/logr/LICENSE; Windows desktop/installer)`

````text
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "{}"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright {yyyy} {name of copyright owner}

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module inline license

Source files: `github.com/go-logr/logr@v1.4.4/funcr/funcr.go`

````text
/*
Copyright 2021 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package funcr implements formatting of structured log messages and
// optionally captures the call site and timestamp.
//
// The simplest way to use it is via its implementation of a
// github.com/go-logr/logr.LogSink with output through an arbitrary
// "write" function.  See New and NewJSON for details.
//
// # Custom LogSinks
//
// For users who need more control, a funcr.Formatter can be embedded inside
// your own custom LogSink implementation. This is useful when the LogSink
// needs to implement additional methods, for example.
//
// # Formatting
//
// This will respect logr.Marshaler, fmt.Stringer, and error interfaces for
// values which are being logged.  When rendering a struct, funcr will use Go's
// standard JSON tags (all except "string").
````

### Go module inline license

Source files: `github.com/go-logr/logr@v1.4.4/logr.go`

````text
/*
Copyright 2019 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This design derives from Dave Cheney's blog:
//     http://dave.cheney.net/2015/11/05/lets-talk-about-logging

// Package logr defines a general-purpose logging API and abstract interfaces
// to back that API.  Packages in the Go ecosystem can depend on this package,
// while callers can implement logging with whatever backend is appropriate.
//
// # Usage
//
// Logging is done using a Logger instance.  Logger is a concrete type with
// methods, which defers the actual logging to a LogSink interface.  The main
// methods of Logger are Info() and Error().  Arguments to Info() and Error()
// are key/value pairs rather than printf-style formatted strings, emphasizing
// "structured logging".
//
// With Go's standard log package, we might write:
//
//	log.Printf("setting target value %s", targetValue)
//
// With logr's structured logging, we'd write:
//
//	logger.Info("setting target", "value", targetValue)
//
// Errors are much the same.  Instead of:
//
//	log.Printf("failed to open the pod bay door for user %s: %v", user, err)
//
// We'd write:
//
//	logger.Error(err, "failed to open the pod bay door", "user", user)
//
// Info() and Error() are very similar, but they are separate methods so that
// LogSink implementations can choose to do things like attach additional
// information (such as stack traces) on calls to Error(). Error() messages are
// always logged, regardless of the current verbosity.  If there is no error
// instance available, passing nil is valid.
//
// # Verbosity
//
// Often we want to log information only when the application in "verbose
// mode".  To write log lines that are more verbose, Logger has a V() method.
// The higher the V-level of a log line, the less critical it is considered.
// Log-lines with V-levels that are not enabled (as per the LogSink) will not
// be written.  Level V(0) is the default, and logger.V(0).Info() has the same
// meaning as logger.Info().  Negative V-levels have the same meaning as V(0).
// Error messages do not have a verbosity level and are always logged.
//
// Where we might have written:
//
//	if flVerbose >= 2 {
//	    log.Printf("an unusual thing happened")
//	}
//
// We can write:
//
//	logger.V(2).Info("an unusual thing happened")
//
// # Logger Names
//
// Logger instances can have name strings so that all messages logged through
// that instance have additional context.  For example, you might want to add
// a subsystem name:
//
//	logger.WithName("compactor").Info("started", "time", time.Now())
//
// The WithName() method returns a new Logger, which can be passed to
// constructors or other functions for further use.  Repeated use of WithName()
// will accumulate name "segments".  These name segments will be joined in some
// way by the LogSink implementation.  It is strongly recommended that name
// segments contain simple identifiers (letters, digits, and hyphen), and do
// not contain characters that could muddle the log output or confuse the
// joining operation (e.g. whitespace, commas, periods, slashes, brackets,
// quotes, etc).
//
// # Saved Values
//
// Logger instances can store any number of key/value pairs, which will be
// logged alongside all messages logged through that instance.  For example,
// you might want to create a Logger instance per managed object:
//
// With the standard log package, we might write:
//
//	log.Printf("decided to set field foo to value %q for object %s/%s",
//	    targetValue, object.Namespace, object.Name)
//
// With logr we'd write:
//
//	// Elsewhere: set up the logger to log the object name.
//	obj.logger = mainLogger.WithValues(
//	    "name", obj.name, "namespace", obj.namespace)
//
//	// later on...
//	obj.logger.Info("setting foo", "value", targetValue)
//
// # Best Practices
//
// Logger has very few hard rules, with the goal that LogSink implementations
// might have a lot of freedom to differentiate.  There are, however, some
// things to consider.
//
// The log message consists of a constant message attached to the log line.
// This should generally be a simple description of what's occurring, and should
// never be a format string.  Variable information can then be attached using
// named values.
//
// Keys are arbitrary strings, but should generally be constant values.  Values
// may be any Go value, but how the value is formatted is determined by the
// LogSink implementation.
//
// Logger instances are meant to be passed around by value. Code that receives
// such a value can call its methods without having to check whether the
// instance is ready for use.
//
// The zero logger (= Logger{}) is identical to Discard() and discards all log
// entries. Code that receives a Logger by value can simply call it, the methods
// will never crash. For cases where passing a logger is optional, a pointer to Logger
// should be used.
//
// # Key Naming Conventions
//
// Keys are not strictly required to conform to any specification or regex, but
// it is recommended that they:
//   - be human-readable and meaningful (not auto-generated or simple ordinals)
//   - be constant (not dependent on input data)
//   - contain only printable characters
//   - not contain whitespace or punctuation
//   - use lower case for simple keys and lowerCamelCase for more complex ones
//
// These guidelines help ensure that log data is processed properly regardless
// of the log implementation.  For example, log implementations will try to
// output JSON data or will store data for later database (e.g. SQL) queries.
//
// While users are generally free to use key names of their choice, it's
// generally best to avoid using the following keys, as they're frequently used
// by implementations:
//   - "caller": the calling information (file/line) of a particular log line
//   - "error": the underlying error value in the `Error` method
//   - "level": the log level
//   - "logger": the name of the associated logger
//   - "msg": the log message
//   - "stacktrace": the stack trace associated with a particular log line or
//     error (often from the `Error` message)
//   - "ts": the timestamp for a log line
//
// Implementations are encouraged to make use of these keys to represent the
// above concepts, when necessary (for example, in a pure-JSON output form, it
// would be necessary to represent at least message and timestamp as ordinary
// named values).
//
// # Break Glass
//
// Implementations may choose to give callers access to the underlying
// logging implementation.  The recommended pattern for this is:
//
//	// Underlier exposes access to the underlying logging implementation.
//	// Since callers only have a logr.Logger, they have to know which
//	// implementation is in use, so this interface is less of an abstraction
//	// and more of way to test type conversion.
//	type Underlier interface {
//	    GetUnderlying() <underlying-type>
//	}
//
// Logger grants access to the sink to enable type assertions like this:
//
//	func DoSomethingWithImpl(log logr.Logger) {
//	    if underlier, ok := log.GetSink().(impl.Underlier); ok {
//	       implLogger := underlier.GetUnderlying()
//	       ...
//	    }
//	}
//
// Custom `With*` functions can be implemented by copying the complete
// Logger struct and replacing the sink in the copy:
//
//	// WithFooBar changes the foobar parameter in the log sink and returns a
//	// new logger with that modified sink.  It does nothing for loggers where
//	// the sink doesn't support that parameter.
//	func WithFoobar(log logr.Logger, foobar int) logr.Logger {
//	   if foobarLogSink, ok := log.GetSink().(FoobarSink); ok {
//	      log = log.WithSink(foobarLogSink.WithFooBar(foobar))
//	   }
//	   return log
//	}
//
// Don't use New to construct a new Logger with a LogSink retrieved from an
// existing Logger. Source code attribution might not work correctly and
// unexported fields in Logger get lost.
//
// Beware that the same LogSink instance may be shared by different logger
// instances. Calling functions that modify the LogSink will affect all of
// those.
````

### Go module inline license

Source files: `github.com/go-logr/logr@v1.4.4/slogr/slogr.go`

````text
//go:build go1.21

/*
Copyright 2023 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package slogr enables usage of a slog.Handler with logr.Logger as front-end
// API and of a logr.LogSink through the slog.Handler and thus slog.Logger
// APIs.
//
// See the README in the top-level [./logr] package for a discussion of
// interoperability.
//
// Deprecated: use the main logr package instead.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/go-logr/stdr@v1.2.2 (github.com/go-logr/stdr/LICENSE)`, `github.com/go-logr/stdr@v1.2.2 (github.com/go-logr/stdr/LICENSE; Windows desktop/installer)`, `go.opentelemetry.io/auto/sdk@v1.2.1 (go.opentelemetry.io/auto/sdk/LICENSE)`, `go.opentelemetry.io/auto/sdk@v1.2.1 (go.opentelemetry.io/auto/sdk/LICENSE; Windows desktop/installer)`

````text
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright [yyyy] [name of copyright owner]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/go-ole/go-ole@v1.3.0 (github.com/go-ole/go-ole/LICENSE)`, `github.com/go-ole/go-ole@v1.3.0 (github.com/go-ole/go-ole/LICENSE; Windows desktop/installer)`

````text
The MIT License (MIT)

Copyright © 2013-2017 Yasuhiro Matsumoto, <mattn.jp@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the “Software”), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module inline license

Source files: `github.com/go-ole/go-ole@v1.3.0/_example/libreoffice/libreoffice.go`

````text
// +build windows

/*
	Demonstrates basic LibreOffce (OpenOffice) automation with OLE using GO-OLE.
	Usage: 	cd [...]\go-ole\example\libreoffice
			go run libreoffice.go
	References:
			http://www.openoffice.org/api/basic/man/tutorial/tutorial.pdf
			http://api.libreoffice.org/examples/examples.html#OLE_examples
			https://wiki.openoffice.org/wiki/Documentation/BASIC_Guide

	Tested environment:
			go 1.6.2 (windows/amd64)
			LibreOffice 5.1.0.3 (32 bit)
			Windows 10 (64 bit)

	The MIT License (MIT)
	Copyright (c) 2016 Sebastian Schleemilch <https://github.com/itschleemilch>.

	Permission is hereby granted, free of charge, to any person obtaining a copy of
	this software and associated documentation files (the "Software"), to deal in
	the Software without restriction, including without limitation the rights to use,
	copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software,
	and to permit persons to whom the Software is furnished to do so, subject to the
	following conditions:

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
	INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
	PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
	LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE
	OR OTHER DEALINGS IN THE SOFTWARE.
*/
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/go-stack/stack@v1.8.1 (github.com/go-stack/stack/LICENSE.md)`, `github.com/go-stack/stack@v1.8.1 (github.com/go-stack/stack/LICENSE.md; Windows desktop/installer)`

````text
The MIT License (MIT)

Copyright (c) 2014 Chris Hines

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; Android gomobile AAR)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; Linux WebUI arm64)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; Linux WebUI)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; Windows desktop/installer)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; macOS desktop arm64)`, `github.com/mattn/go-colorable@v0.1.16 (github.com/mattn/go-colorable/LICENSE; macOS desktop)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; Android gomobile AAR)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; Linux WebUI arm64)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; Linux WebUI)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; Windows desktop/installer)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; macOS desktop arm64)`, `github.com/mattn/go-runewidth@v0.0.31 (github.com/mattn/go-runewidth/LICENSE; macOS desktop)`

````text
The MIT License (MIT)

Copyright (c) 2016 Yasuhiro Matsumoto

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; Android gomobile AAR)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; Linux WebUI arm64)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; Linux WebUI)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; Windows desktop/installer)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; macOS desktop arm64)`, `github.com/mattn/go-isatty@v0.0.24 (github.com/mattn/go-isatty/LICENSE; macOS desktop)`

````text
Copyright (c) Yasuhiro MATSUMOTO <mattn.jp@gmail.com>

MIT License (Expat)

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/oxtoacart/bpool@v0.0.0-20190530202638-03653db5a59c (github.com/oxtoacart/bpool/LICENSE)`, `github.com/oxtoacart/bpool@v0.0.0-20190530202638-03653db5a59c (github.com/oxtoacart/bpool/LICENSE; Windows desktop/installer)`

````text

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright 2014 Percy Wegmann

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; Android gomobile AAR)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; Linux WebUI arm64)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; Linux WebUI)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; Windows desktop/installer)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; macOS desktop arm64)`, `github.com/quic-go/qpack@v0.6.0 (github.com/quic-go/qpack/LICENSE.md; macOS desktop)`

````text
Copyright 2019 Marten Seemann

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; Android gomobile AAR)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; Linux WebUI arm64)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; Linux WebUI)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; Windows desktop/installer)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; macOS desktop arm64)`, `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/LICENSE; macOS desktop)`

````text
MIT License

Copyright (c) 2016 the quic-go authors & Google, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module supplemental license/notice

Source files: `github.com/quic-go/quic-go@v0.65.0 (github.com/quic-go/quic-go/assets/LICENSE.md)`

````text
# quic-go Logo and Trademark Usage Policy

## Exception to Main License

The files in this directory (collectively, "Brand Assets") are **excluded** from the quic-go project's main MIT License. These assets are protected by copyright and trademark laws.

## Permitted Use

You are granted a limited, non-exclusive license to use these Brand Assets solely for the following purposes:

- **Editorial and Press:** You may use the Brand Assets in blog posts, news articles, video reviews, and public presentations that discuss, review, or reference the quic-go project.

- **Reference:** You may use the Brand Assets to indicate your project's compatibility with or dependence on quic-go (e.g., "Powered by quic-go").

## Restricted Use

You may NOT:

- **Modification:** Modify the Brand Assets in any way (including changing colors, aspect ratio, or obscuring the image). Resizing the image while maintaining the original aspect ratio is permitted.

- **No Branding:** Use the Brand Assets as the logo, icon, or mascot for your own project, product, or service.

- **No Endorsement:** Use the Brand Assets in a way that suggests your project is officially sponsored by, endorsed by, or affiliated with the quic-go maintainers.

## Termination

The quic-go project reserves the right to revoke this authorization at any time if usage is found to be confusing, misleading, or detrimental to the project's reputation.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/internal/go-common-file-dialog/LICENSE)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/internal/go-common-file-dialog/cfd/LICENSE; Windows desktop/installer)`

````text
MIT License

Copyright (c) 2019 Harry Phillips

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module supplemental license/notice

Source files: `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/internal/templates/ios/frontend/public/puppertino/LICENSE)`

````text
MIT License

Copyright (c) 2019 Edgar Pérez

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### Go module license, Go module supplemental license/notice

Source files: `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/internal/webview2/webviewloader/LICENSE)`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103 (github.com/wailsapp/wails/v3/internal/webview2/webviewloader/LICENSE; Windows desktop/installer)`

````text
ISC License (ISC)

Copyright (c) 2020 John Chadwick
Copyright (c) 2022 Wails Project Developers

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.
````

### Go module inline license

Source files: `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103/internal/assetserver/ringqueue.go`

````text
// Code from https://github.com/erikdubbelboer/ringqueue
/*
The MIT License (MIT)

Copyright (c) 2015 Erik Dubbelboer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
````

### Go module inline license

Source files: `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103/internal/webview2/pkg/edge/guid.go`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103/internal/webview2/pkg/webview2/com.go`, `github.com/wailsapp/wails/v3@v3.2.0-nightly.20261103/pkg/w32/guid.go`

````text
// This code has been adapted from: https://github.com/go-ole/go-ole

/*

The MIT License (MIT)

Copyright © 2013-2017 Yasuhiro Matsumoto, <mattn.jp@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the “Software”), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/
````

### Go module inline license

Source files: `go.opentelemetry.io/auto/sdk@v1.2.1/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package sdk provides an auto-instrumentable OpenTelemetry SDK.

An [go.opentelemetry.io/auto.Instrumentation] can be configured to target the
process running this SDK. In that case, all telemetry the SDK produces will be
processed and handled by that [go.opentelemetry.io/auto.Instrumentation].

By default, if there is no [go.opentelemetry.io/auto.Instrumentation] set to
auto-instrument the SDK, the SDK will not generate any telemetry.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/attr.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/id.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/number.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/resource.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/scope.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/span.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/status.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/traces.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/value.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/limit.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/span.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/tracer.go`, `go.opentelemetry.io/auto/sdk@v1.2.1/tracer_provider.go`, `go.opentelemetry.io/otel/log@v1.47.0/logger.go`, `go.opentelemetry.io/otel/log@v1.47.0/provider.go`, `go.opentelemetry.io/otel/log@v1.47.0/record.go`, `go.opentelemetry.io/otel/metric@v1.47.0/asyncfloat64.go`, `go.opentelemetry.io/otel/metric@v1.47.0/asyncint64.go`, `go.opentelemetry.io/otel/metric@v1.47.0/config.go`, `go.opentelemetry.io/otel/metric@v1.47.0/instrument.go`, `go.opentelemetry.io/otel/metric@v1.47.0/meter.go`, `go.opentelemetry.io/otel/metric@v1.47.0/syncfloat64.go`, `go.opentelemetry.io/otel/metric@v1.47.0/syncint64.go`, `go.opentelemetry.io/otel/trace@v1.47.0/auto.go`, `go.opentelemetry.io/otel/trace@v1.47.0/config.go`, `go.opentelemetry.io/otel/trace@v1.47.0/context.go`, `go.opentelemetry.io/otel/trace@v1.47.0/hex.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/attr.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/id.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/number.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/resource.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/scope.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/span.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/status.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/traces.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/value.go`, `go.opentelemetry.io/otel/trace@v1.47.0/nonrecording.go`, `go.opentelemetry.io/otel/trace@v1.47.0/noop.go`, `go.opentelemetry.io/otel/trace@v1.47.0/provider.go`, `go.opentelemetry.io/otel/trace@v1.47.0/span.go`, `go.opentelemetry.io/otel/trace@v1.47.0/trace.go`, `go.opentelemetry.io/otel/trace@v1.47.0/tracer.go`, `go.opentelemetry.io/otel/trace@v1.47.0/tracestate.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/encoder.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/filter.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/hash.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/iterator.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/key.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/kv.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/rawhelpers.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/set.go`, `go.opentelemetry.io/otel@v1.47.0/attribute/value.go`, `go.opentelemetry.io/otel@v1.47.0/baggage/baggage.go`, `go.opentelemetry.io/otel@v1.47.0/baggage/context.go`, `go.opentelemetry.io/otel@v1.47.0/codes/codes.go`, `go.opentelemetry.io/otel@v1.47.0/error_handler.go`, `go.opentelemetry.io/otel@v1.47.0/handler.go`, `go.opentelemetry.io/otel@v1.47.0/internal/baggage/context.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/instruments.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/internal_logging.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/logger.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/meter.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/propagator.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/state.go`, `go.opentelemetry.io/otel@v1.47.0/internal/global/trace.go`, `go.opentelemetry.io/otel@v1.47.0/internal/internaltest/text_map_carrier.go`, `go.opentelemetry.io/otel@v1.47.0/internal/internaltest/text_map_propagator.go`, `go.opentelemetry.io/otel@v1.47.0/internal_logging.go`, `go.opentelemetry.io/otel@v1.47.0/log.go`, `go.opentelemetry.io/otel@v1.47.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/propagation.go`, `go.opentelemetry.io/otel@v1.47.0/propagation/baggage.go`, `go.opentelemetry.io/otel@v1.47.0/propagation/propagation.go`, `go.opentelemetry.io/otel@v1.47.0/propagation/trace_context.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v2/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v3/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v4/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.26.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.26.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.27.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.27.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.28.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.28.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.30.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.30.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.31.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.31.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/version.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
````

### Go module inline license

Source files: `go.opentelemetry.io/auto/sdk@v1.2.1/internal/telemetry/doc.go`, `go.opentelemetry.io/otel/trace@v1.47.0/internal/telemetry/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package telemetry provides a lightweight representations of OpenTelemetry
telemetry that is compatible with the OTLP JSON protobuf encoding.
*/
````

### Go module license, Go module supplemental license/notice

Source files: `go.opentelemetry.io/otel/log@v1.47.0 (go.opentelemetry.io/otel/log/LICENSE)`, `go.opentelemetry.io/otel/log@v1.47.0 (go.opentelemetry.io/otel/log/LICENSE; Windows desktop/installer)`, `go.opentelemetry.io/otel/metric@v1.47.0 (go.opentelemetry.io/otel/metric/LICENSE)`, `go.opentelemetry.io/otel/metric@v1.47.0 (go.opentelemetry.io/otel/metric/LICENSE; Windows desktop/installer)`, `go.opentelemetry.io/otel/trace@v1.47.0 (go.opentelemetry.io/otel/trace/LICENSE)`, `go.opentelemetry.io/otel/trace@v1.47.0 (go.opentelemetry.io/otel/trace/LICENSE; Windows desktop/installer)`, `go.opentelemetry.io/otel@v1.47.0 (go.opentelemetry.io/otel/LICENSE)`, `go.opentelemetry.io/otel@v1.47.0 (go.opentelemetry.io/otel/LICENSE; Windows desktop/installer)`

````text
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright [yyyy] [name of copyright owner]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

--------------------------------------------------------------------------------

Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/log@v1.47.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package log provides the OpenTelemetry Logs API.

This API is separate from its implementation, so the instrumentation built
from it is reusable. See [go.opentelemetry.io/otel/sdk/log] for the official
OpenTelemetry implementation of this API.

The log package provides the OpenTelemetry Logs API, which serves as a standard
interface for generating and managing log records within the OpenTelemetry ecosystem.
This package allows users to emit LogRecords, enabling structured, context-rich logging
that can be easily integrated with observability tools. It ensures that log data is captured
in a way that is consistent with OpenTelemetry's data model.

This package can be used to create bridges between existing logging libraries and OpenTelemetry.
Log bridges allow existing logging setups to be integrated with OpenTelemetry.
Log bridges can be found in the [registry].

# API Implementations

This package does not conform to the standard Go versioning policy; all of its
interfaces may have methods added to them without a package major version bump.
This non-standard API evolution could surprise an uninformed implementation
author. They could unknowingly build their implementation in a way that would
result in a runtime panic for their users who update to the new API.

The API is designed to help inform an instrumentation author about this
non-standard API evolution. It requires them to choose a default behavior for
unimplemented interface methods. There are three behavior choices they can
make:

  - Compilation failure
  - Panic
  - Default to another implementation

All interfaces in this API embed a corresponding interface from
[go.opentelemetry.io/otel/log/embedded]. If an author wants the default
behavior of their implementations to be a compilation failure, signaling to
their users that they need to update to the latest version of that implementation,
they need to embed the corresponding interface from
[go.opentelemetry.io/otel/log/embedded] in their implementation. For example,

	import "go.opentelemetry.io/otel/log/embedded"

	type LoggerProvider struct {
		embedded.LoggerProvider
		// ...
	}

If an author wants the default behavior of their implementations to panic,
they need to embed the API interface directly.

	import "go.opentelemetry.io/otel/log"

	type LoggerProvider struct {
		log.LoggerProvider
		// ...
	}

This is not a recommended behavior as it could lead to publishing packages that
contain runtime panics when users update other packages that use newer versions
of [go.opentelemetry.io/otel/log].

Finally, an author can embed another implementation in theirs. The embedded
implementation will be used for methods not defined by the author. For example,
an author who wants to default to silently dropping the call can use
[go.opentelemetry.io/otel/log/noop]:

	import "go.opentelemetry.io/otel/log/noop"

	type LoggerProvider struct {
		noop.LoggerProvider
		// ...
	}

It is strongly recommended that authors only embed
go.opentelemetry.io/otel/log/noop if they choose this default behavior. That
implementation is the only one OpenTelemetry authors can guarantee will fully
implement all the API interfaces when a user updates their API.

[registry]: https://opentelemetry.io/ecosystem/registry/?language=go&component=log-bridge
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/log@v1.47.0/embedded/embedded.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package embedded provides interfaces embedded within the [OpenTelemetry Logs
// Bridge API].
//
// Implementers of the [OpenTelemetry Logs API] can embed the relevant
// type from this package into their implementation directly. Doing so will
// result in a compilation error for users when the [OpenTelemetry Logs Bridge
// API] is extended (which is something that can happen without a major version
// bump of the API package).
//
// [OpenTelemetry Logs API]: https://pkg.go.dev/go.opentelemetry.io/otel/log
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/log@v1.47.0/global/log.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package global provides access to a global implementation of the OpenTelemetry
Logs API.

Deprecated: Use the equivalent APIs in [go.opentelemetry.io/otel].
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/log@v1.47.0/noop/noop.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package noop provides an implementation of the [OpenTelemetry Logs Bridge
// API] that produces no telemetry and minimizes computational resource usage.
//
// Using this package to implement the [OpenTelemetry Logs API] will
// effectively disable OpenTelemetry.
//
// This implementation can be embedded in other implementations of the
// [OpenTelemetry Logs API]. Doing so means that the implementation defaults to
// performing no operation for methods it does not implement.
//
// [OpenTelemetry Logs API]: https://pkg.go.dev/go.opentelemetry.io/otel/log
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/log@v1.47.0/severity.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate stringer -type=Severity -linecomment
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/metric@v1.47.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package metric provides the OpenTelemetry API used to measure metrics about
source code operation.

This API is separate from its implementation so the instrumentation built from
it is reusable. See [go.opentelemetry.io/otel/sdk/metric] for the official
OpenTelemetry implementation of this API.

All measurements made with this package are made via instruments. These
instruments are created by a [Meter] which itself is created by a
[MeterProvider]. Applications need to accept a [MeterProvider] implementation
as a starting point when instrumenting. This can be done directly, or by using
the OpenTelemetry global MeterProvider via [GetMeterProvider]. Using an
appropriately named [Meter] from the accepted [MeterProvider], instrumentation
can then be built from the [Meter]'s instruments.

# Instruments

Each instrument is designed to make measurements of a particular type. Broadly,
all instruments fall into two overlapping logical categories: asynchronous or
synchronous, and int64 or float64.

All synchronous instruments ([Int64Counter], [Int64UpDownCounter],
[Int64Histogram], [Int64Gauge], [Float64Counter], [Float64UpDownCounter],
[Float64Histogram], and [Float64Gauge]) are used to measure the operation and
performance of source code during the source code execution. These instruments
only make measurements when the source code they instrument is run.

All asynchronous instruments ([Int64ObservableCounter],
[Int64ObservableUpDownCounter], [Int64ObservableGauge],
[Float64ObservableCounter], [Float64ObservableUpDownCounter], and
[Float64ObservableGauge]) are used to measure metrics outside of the execution
of source code. They are said to make "observations" via a callback function
called once every measurement collection cycle.

Each instrument is also grouped by the value type it measures. Either int64 or
float64. The value being measured will dictate which instrument in these
categories to use.

Outside of these two broad categories, instruments are described by the
function they are designed to serve. All Counters ([Int64Counter],
[Float64Counter], [Int64ObservableCounter], and [Float64ObservableCounter]) are
designed to measure values that never decrease in value, but instead only
incrementally increase in value. UpDownCounters ([Int64UpDownCounter],
[Float64UpDownCounter], [Int64ObservableUpDownCounter], and
[Float64ObservableUpDownCounter]) on the other hand, are designed to measure
values that can increase and decrease. When more information needs to be
conveyed about all the synchronous measurements made during a collection cycle,
a Histogram ([Int64Histogram] and [Float64Histogram]) should be used. Finally,
when just the most recent measurement needs to be conveyed, a Gauge
([Int64Gauge], [Float64Gauge], [Int64ObservableGauge], and
[Float64ObservableGauge]) should be used: the synchronous variants record an
instantaneous value at a specific point in code, while the observable variants
sample the value via a callback once per collection cycle.

See the [OpenTelemetry documentation] for more information about instruments
and their intended use.

# Instrument Name

OpenTelemetry defines an [instrument name syntax] that restricts what
instrument names are allowed.

Instrument names should ...

  - Not be empty.
  - Have an alphabetic character as their first letter.
  - Have any letter after the first be an alphanumeric character, ‘_’, ‘.’,
    ‘-’, or ‘/’.
  - Have a maximum length of 255 letters.

To ensure compatibility with observability platforms, all instruments created
need to conform to this syntax. Not all implementations of the API will validate
these names, it is the callers responsibility to ensure compliance.

# Measurements

Measurements are made by recording values and information about the values with
an instrument. How these measurements are recorded depends on the instrument.

Measurements for synchronous instruments ([Int64Counter], [Int64UpDownCounter],
[Int64Histogram], [Int64Gauge], [Float64Counter], [Float64UpDownCounter],
[Float64Histogram], and [Float64Gauge]) are recorded using the instrument
methods directly. All counter instruments have an Add method that is used to
measure an increment value, and all histogram and synchronous gauge
instruments have a Record method to measure a data point.

Asynchronous instruments ([Int64ObservableCounter],
[Int64ObservableUpDownCounter], [Int64ObservableGauge],
[Float64ObservableCounter], [Float64ObservableUpDownCounter], and
[Float64ObservableGauge]) record measurements within a callback function. The
callback is registered with the Meter which ensures the callback is called once
per collection cycle. A callback can be registered two ways: during the
instrument's creation using an option, or later using the RegisterCallback
method of the [Meter] that created the instrument.

If the following criteria are met, an option ([WithInt64Callback] or
[WithFloat64Callback]) can be used during the asynchronous instrument's
creation to register a callback ([Int64Callback] or [Float64Callback],
respectively):

  - The measurement process is known when the instrument is created
  - Only that instrument will make a measurement within the callback
  - The callback never needs to be unregistered

If the criteria are not met, use the RegisterCallback method of the [Meter] that
created the instrument to register a [Callback].

# Avoiding Expensive Computations

All synchronous instruments provide an Enabled method that reports whether the
instrument will process measurements for the given context. When no SDK is
registered or the instrument is otherwise disabled, Enabled returns false. This
can be used to avoid expensive measurement work when a measurement will not be
recorded:

	if counter.Enabled(ctx) {
		counter.Add(ctx, 1, metric.WithAttributes(expensiveAttributes()...))
	}

This is especially valuable when computing attributes is expensive.
[WithAttributes] performs non-trivial work on every call to build an
[attribute.Set] from the provided attributes, and that work is wasted if the
measurement is not recorded.

For performance sensitive code where the same attribute set is used repeatedly,
prefer [WithAttributeSet]. It accepts a pre-built [attribute.Set], letting you
pay the construction cost once and reuse it across many measurements:

	attrs := attribute.NewSet(attribute.String("key", "val"))
	// ... later, on each call:
	counter.Add(ctx, 1, metric.WithAttributeSet(attrs))

# API Implementations

This package does not conform to the standard Go versioning policy, all of its
interfaces may have methods added to them without a package major version bump.
This non-standard API evolution could surprise an uninformed implementation
author. They could unknowingly build their implementation in a way that would
result in a runtime panic for their users that update to the new API.

The API is designed to help inform an instrumentation author about this
non-standard API evolution. It requires them to choose a default behavior for
unimplemented interface methods. There are three behavior choices they can
make:

  - Compilation failure
  - Panic
  - Default to another implementation

All interfaces in this API embed a corresponding interface from
[go.opentelemetry.io/otel/metric/embedded]. If an author wants the default
behavior of their implementations to be a compilation failure, signaling to
their users they need to update to the latest version of that implementation,
they need to embed the corresponding interface from
[go.opentelemetry.io/otel/metric/embedded] in their implementation. For
example,

	import "go.opentelemetry.io/otel/metric/embedded"

	type MeterProvider struct {
		embedded.MeterProvider
		// ...
	}

If an author wants the default behavior of their implementations to a panic,
they need to embed the API interface directly.

	import "go.opentelemetry.io/otel/metric"

	type MeterProvider struct {
		metric.MeterProvider
		// ...
	}

This is not a recommended behavior as it could lead to publishing packages that
contain runtime panics when users update other package that use newer versions
of [go.opentelemetry.io/otel/metric].

Finally, an author can embed another implementation in theirs. The embedded
implementation will be used for methods not defined by the author. For example,
an author who wants to default to silently dropping the call can use
[go.opentelemetry.io/otel/metric/noop]:

	import "go.opentelemetry.io/otel/metric/noop"

	type MeterProvider struct {
		noop.MeterProvider
		// ...
	}

It is strongly recommended that authors only embed
[go.opentelemetry.io/otel/metric/noop] if they choose this default behavior.
That implementation is the only one OpenTelemetry authors can guarantee will
fully implement all the API interfaces when a user updates their API.

[instrument name syntax]: https://opentelemetry.io/docs/specs/otel/metrics/api/#instrument-name-syntax
[OpenTelemetry documentation]: https://opentelemetry.io/docs/concepts/signals/metrics/
[GetMeterProvider]: https://pkg.go.dev/go.opentelemetry.io/otel#GetMeterProvider
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/metric@v1.47.0/embedded/embedded.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package embedded provides interfaces embedded within the [OpenTelemetry
// metric API].
//
// Implementers of the [OpenTelemetry metric API] can embed the relevant type
// from this package into their implementation directly. Doing so will result
// in a compilation error for users when the [OpenTelemetry metric API] is
// extended (which is something that can happen without a major version bump of
// the API package).
//
// [OpenTelemetry metric API]: https://pkg.go.dev/go.opentelemetry.io/otel/metric
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/metric@v1.47.0/noop/noop.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package noop provides an implementation of the OpenTelemetry metric API that
// produces no telemetry and minimizes used computation resources.
//
// Using this package to implement the OpenTelemetry metric API will
// effectively disable OpenTelemetry.
//
// This implementation can be embedded in other implementations of the
// OpenTelemetry metric API. Doing so will mean the implementation defaults to
// no operation for methods it does not implement.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/trace@v1.47.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package trace provides an implementation of the tracing part of the
OpenTelemetry API.

To participate in distributed traces a Span needs to be created for the
operation being performed as part of a traced workflow. In its simplest form:

	var tracer trace.Tracer

	func init() {
		tracer = otel.Tracer("instrumentation/package/name")
	}

	func operation(ctx context.Context) {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "operation")
		defer span.End()
		// ...
	}

A Tracer is unique to the instrumentation and is used to create Spans.
Instrumentation should be designed to accept a TracerProvider from which it
can create its own unique Tracer. Alternatively, the registered global
TracerProvider from the go.opentelemetry.io/otel package can be used as
a default.

	const (
		name    = "instrumentation/package/name"
		version = "0.1.0"
	)

	type Instrumentation struct {
		tracer trace.Tracer
	}

	func NewInstrumentation(tp trace.TracerProvider) *Instrumentation {
		if tp == nil {
			tp = otel.TracerProvider()
		}
		return &Instrumentation{
			tracer: tp.Tracer(name, trace.WithInstrumentationVersion(version)),
		}
	}

	func operation(ctx context.Context, inst *Instrumentation) {
		var span trace.Span
		ctx, span = inst.tracer.Start(ctx, "operation")
		defer span.End()
		// ...
	}

# API Implementations

This package does not conform to the standard Go versioning policy; all of its
interfaces may have methods added to them without a package major version bump.
This non-standard API evolution could surprise an uninformed implementation
author. They could unknowingly build their implementation in a way that would
result in a runtime panic for their users that update to the new API.

The API is designed to help inform an instrumentation author about this
non-standard API evolution. It requires them to choose a default behavior for
unimplemented interface methods. There are three behavior choices they can
make:

  - Compilation failure
  - Panic
  - Default to another implementation

All interfaces in this API embed a corresponding interface from
[go.opentelemetry.io/otel/trace/embedded]. If an author wants the default
behavior of their implementations to be a compilation failure, signaling to
their users they need to update to the latest version of that implementation,
they need to embed the corresponding interface from
[go.opentelemetry.io/otel/trace/embedded] in their implementation. For
example,

	import "go.opentelemetry.io/otel/trace/embedded"

	type TracerProvider struct {
		embedded.TracerProvider
		// ...
	}

If an author wants the default behavior of their implementations to panic, they
can embed the API interface directly.

	import "go.opentelemetry.io/otel/trace"

	type TracerProvider struct {
		trace.TracerProvider
		// ...
	}

This option is not recommended. It will lead to publishing packages that
contain runtime panics when users update to newer versions of
[go.opentelemetry.io/otel/trace], which may be done with a transitive
dependency.

Finally, an author can embed another implementation in theirs. The embedded
implementation will be used for methods not defined by the author. For example,
an author who wants to default to silently dropping the call can use
[go.opentelemetry.io/otel/trace/noop]:

	import "go.opentelemetry.io/otel/trace/noop"

	type TracerProvider struct {
		noop.TracerProvider
		// ...
	}

It is strongly recommended that authors only embed
[go.opentelemetry.io/otel/trace/noop] if they choose this default behavior.
That implementation is the only one OpenTelemetry authors can guarantee will
fully implement all the API interfaces when a user updates their API.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/trace@v1.47.0/embedded/embedded.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package embedded provides interfaces embedded within the [OpenTelemetry
// trace API].
//
// Implementers of the [OpenTelemetry trace API] can embed the relevant type
// from this package into their implementation directly. Doing so will result
// in a compilation error for users when the [OpenTelemetry trace API] is
// extended (which is something that can happen without a major version bump of
// the API package).
//
// [OpenTelemetry trace API]: https://pkg.go.dev/go.opentelemetry.io/otel/trace
````

### Go module inline license

Source files: `go.opentelemetry.io/otel/trace@v1.47.0/noop/noop.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package noop provides an implementation of the OpenTelemetry trace API that
// produces no telemetry and minimizes used computation resources.
//
// Using this package to implement the OpenTelemetry trace API will effectively
// disable OpenTelemetry.
//
// This implementation can be embedded in other implementations of the
// OpenTelemetry trace API. Doing so will mean the implementation defaults to
// no operation for methods it does not implement.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/attribute/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package attribute provides key and value attributes.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/attribute/internal/attribute.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package attribute provide several helper functions for some commonly used
logic of processing attributes.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/attribute/internal/xxhash/xxhash.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package xxhash provides a wrapper around the xxhash library for attribute hashing.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/baggage/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package baggage provides functionality for storing and retrieving
baggage items in Go context. For propagating the baggage, see the
go.opentelemetry.io/otel/propagation package.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/codes/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package codes defines the canonical error codes used by OpenTelemetry.

It conforms to [the OpenTelemetry
specification](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.20.0/specification/trace/api.md#set-status).
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package otel provides global access to the OpenTelemetry API. The subpackages of
the otel package provide an implementation of the OpenTelemetry API.

The provided API is used to instrument code and measure data about that code's
performance and operation. The measured data, by default, is not processed or
transmitted anywhere. An implementation of the OpenTelemetry SDK, like the
default SDK implementation (go.opentelemetry.io/otel/sdk), and associated
exporters are used to process and transport this data.

To read the getting started guide, see https://opentelemetry.io/docs/languages/go/getting-started/.

To read more about tracing, see go.opentelemetry.io/otel/trace.

To read more about metrics, see go.opentelemetry.io/otel/metric.

To read more about logs, see go.opentelemetry.io/otel/log.

To read more about propagation, see go.opentelemetry.io/otel/propagation and
go.opentelemetry.io/otel/baggage.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/internal/baggage/baggage.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package baggage provides base types and functionality to store and retrieve
baggage in Go context. This package exists because the OpenTracing bridge to
OpenTelemetry needs to synchronize state whenever baggage for a context is
modified and that context contains an OpenTracing span. If it were not for
this need this package would not need to exist and the
`go.opentelemetry.io/otel/baggage` package would be the singular place where
W3C baggage is handled.
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/internal/errorhandler/errorhandler.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package errorhandler provides the global error handler for OpenTelemetry.
//
// This package has no OTel dependencies, allowing it to be imported by any
// package in the module without creating import cycles.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/internal/global/handler.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package global provides the OpenTelemetry global API.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/internal/internaltest/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internaltest provides testing functionality.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/propagation/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
Package propagation contains OpenTelemetry context propagators.

OpenTelemetry propagators are used to extract and inject context data from and
into messages exchanged by applications. The propagator supported by this
package is the W3C Trace Context encoding
(https://www.w3.org/TR/trace-context/), and W3C Baggage
(https://www.w3.org/TR/baggage/).
*/
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/propagation/internal/hextable/hextable.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package hextable provides a lookup table for decoding lowercase
// hexadecimal.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/internal/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v2/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v3/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/internal/v4/http.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internal provides common semconv functionality.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/internal/metricpool/pool.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package metricpool provides shared pools for semantic convention metric
// measurement options.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.10.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.10.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/event.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.26.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.26.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.27.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.27.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.28.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.28.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.30.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.30.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.31.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.31.0/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/attribute_group.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/trace.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/resource.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/trace.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Code generated from semantic convention specification. DO NOT EDIT.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.11.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.11.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.12.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.12.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.13.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/httpconv/http.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/httpconv/http.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconv provides OpenTelemetry HTTP semantic conventions for
// tracing telemetry.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.13.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/netconv/net.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/netconv/net.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package netconv provides OpenTelemetry network semantic conventions for
// tracing telemetry.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.14.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.14.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.15.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.15.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.16.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.16.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.17.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.17.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.18.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.18.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.19.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.19.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.20.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.20.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.21.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.21.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.22.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.22.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.23.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.23.1/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.23.1
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.24.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.24.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.25.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.25.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.26.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.26.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.27.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.27.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.28.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.28.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.30.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.30.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.31.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.31.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/azureconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/azureconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package azureconv provides types and functionality for OpenTelemetry semantic
// conventions in the "azure" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/cicdconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/cicdconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cicdconv provides types and functionality for OpenTelemetry semantic
// conventions in the "cicd" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/containerconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/containerconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package containerconv provides types and functionality for OpenTelemetry semantic
// conventions in the "container" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/cpuconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/cpuconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/cpuconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cpuconv provides types and functionality for OpenTelemetry semantic
// conventions in the "cpu" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/dbconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/dbconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dbconv provides types and functionality for OpenTelemetry semantic
// conventions in the "db" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/dnsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/dnsconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dnsconv provides types and functionality for OpenTelemetry semantic
// conventions in the "dns" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.32.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/faasconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/faasconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package faasconv provides types and functionality for OpenTelemetry semantic
// conventions in the "faas" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/genaiconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/genaiconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package genaiconv provides types and functionality for OpenTelemetry semantic
// conventions in the "gen_ai" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/goconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/goconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package goconv provides types and functionality for OpenTelemetry semantic
// conventions in the "go" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/httpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/httpconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconv provides types and functionality for OpenTelemetry semantic
// conventions in the "http" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/hwconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/hwconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package hwconv provides types and functionality for OpenTelemetry semantic
// conventions in the "hw" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/k8sconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/k8sconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package k8sconv provides types and functionality for OpenTelemetry semantic
// conventions in the "k8s" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/messagingconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/messagingconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package messagingconv provides types and functionality for OpenTelemetry semantic
// conventions in the "messaging" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/otelconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/otelconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelconv provides types and functionality for OpenTelemetry semantic
// conventions in the "otel" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/processconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/processconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package processconv provides types and functionality for OpenTelemetry semantic
// conventions in the "process" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/rpcconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/rpcconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package rpcconv provides types and functionality for OpenTelemetry semantic
// conventions in the "rpc" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/signalrconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/signalrconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package signalrconv provides types and functionality for OpenTelemetry semantic
// conventions in the "signalr" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/systemconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/systemconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package systemconv provides types and functionality for OpenTelemetry semantic
// conventions in the "system" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.32.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/vcsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/vcsconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package vcsconv provides types and functionality for OpenTelemetry semantic
// conventions in the "vcs" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.33.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.33.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.34.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.34.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.36.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.36.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.37.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.37.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.38.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/nfsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/nfsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/nfsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/nfsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/nfsconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/nfsconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package nfsconv provides types and functionality for OpenTelemetry semantic
// conventions in the "nfs" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.38.0/openshiftconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/openshiftconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/openshiftconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/openshiftconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/openshiftconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/openshiftconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package openshiftconv provides types and functionality for OpenTelemetry semantic
// conventions in the "openshift" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.39.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.39.0/mcpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/mcpconv/metric.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/mcpconv/metric.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package mcpconv provides types and functionality for OpenTelemetry semantic
// conventions in the "mcp" namespace.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.4.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.4.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.40.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.40.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/doc.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.41.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.41.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/schema.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/error_type.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/exception.go`, `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/schema.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.42.0/doc.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.42.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.43.0/doc.go`

````text
// Code generated from semantic convention specification. DO NOT EDIT.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the v1.43.0
// version of the OpenTelemetry semantic conventions.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.5.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.5.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.6.1/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.6.1 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.7.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.7.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.8.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.8.0 version of the OpenTelemetry specification.
````

### Go module inline license

Source files: `go.opentelemetry.io/otel@v1.47.0/semconv/v1.9.0/doc.go`

````text
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconv implements OpenTelemetry semantic conventions.
//
// OpenTelemetry semantic conventions are agreed standardized naming
// patterns for OpenTelemetry things. This package represents the conventions
// as of the v1.9.0 version of the OpenTelemetry specification.
````

### Go module license, Go module supplemental license/notice

Source files: `go.uber.org/multierr@v1.11.0 (go.uber.org/multierr/LICENSE.txt)`, `go.uber.org/multierr@v1.11.0 (go.uber.org/multierr/LICENSE.txt; Windows desktop/installer)`

````text
Copyright (c) 2017-2021 Uber Technologies, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/multierr@v1.11.0/error.go`

````text
// Copyright (c) 2017-2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package multierr allows combining one or more errors together.
//
// # Overview
//
// Errors can be combined with the use of the Combine function.
//
//	multierr.Combine(
//		reader.Close(),
//		writer.Close(),
//		conn.Close(),
//	)
//
// If only two errors are being combined, the Append function may be used
// instead.
//
//	err = multierr.Append(reader.Close(), writer.Close())
//
// The underlying list of errors for a returned error object may be retrieved
// with the Errors function.
//
//	errors := multierr.Errors(err)
//	if len(errors) > 0 {
//		fmt.Println("The following errors occurred:", errors)
//	}
//
// # Appending from a loop
//
// You sometimes need to append into an error from a loop.
//
//	var err error
//	for _, item := range items {
//		err = multierr.Append(err, process(item))
//	}
//
// Cases like this may require knowledge of whether an individual instance
// failed. This usually requires introduction of a new variable.
//
//	var err error
//	for _, item := range items {
//		if perr := process(item); perr != nil {
//			log.Warn("skipping item", item)
//			err = multierr.Append(err, perr)
//		}
//	}
//
// multierr includes AppendInto to simplify cases like this.
//
//	var err error
//	for _, item := range items {
//		if multierr.AppendInto(&err, process(item)) {
//			log.Warn("skipping item", item)
//		}
//	}
//
// This will append the error into the err variable, and return true if that
// individual error was non-nil.
//
// See [AppendInto] for more information.
//
// # Deferred Functions
//
// Go makes it possible to modify the return value of a function in a defer
// block if the function was using named returns. This makes it possible to
// record resource cleanup failures from deferred blocks.
//
//	func sendRequest(req Request) (err error) {
//		conn, err := openConnection()
//		if err != nil {
//			return err
//		}
//		defer func() {
//			err = multierr.Append(err, conn.Close())
//		}()
//		// ...
//	}
//
// multierr provides the Invoker type and AppendInvoke function to make cases
// like the above simpler and obviate the need for a closure. The following is
// roughly equivalent to the example above.
//
//	func sendRequest(req Request) (err error) {
//		conn, err := openConnection()
//		if err != nil {
//			return err
//		}
//		defer multierr.AppendInvoke(&err, multierr.Close(conn))
//		// ...
//	}
//
// See [AppendInvoke] and [Invoker] for more information.
//
// NOTE: If you're modifying an error from inside a defer, you MUST use a named
// return value for that function.
//
// # Advanced Usage
//
// Errors returned by Combine and Append MAY implement the following
// interface.
//
//	type errorGroup interface {
//		// Returns a slice containing the underlying list of errors.
//		//
//		// This slice MUST NOT be modified by the caller.
//		Errors() []error
//	}
//
// Note that if you need access to list of errors behind a multierr error, you
// should prefer using the Errors function. That said, if you need cheap
// read-only access to the underlying errors slice, you can attempt to cast
// the error to this interface. You MUST handle the failure case gracefully
// because errors returned by Combine and Append are not guaranteed to
// implement this interface.
//
//	var errors []error
//	group, ok := err.(errorGroup)
//	if ok {
//		errors = group.Errors()
//	} else {
//		errors = []error{err}
//	}
````

### Go module inline license

Source files: `go.uber.org/multierr@v1.11.0/error_post_go120.go`

````text
// Copyright (c) 2017-2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build go1.20
// +build go1.20
````

### Go module inline license

Source files: `go.uber.org/multierr@v1.11.0/error_pre_go120.go`

````text
// Copyright (c) 2017-2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build !go1.20
// +build !go1.20
````

### Go module license, Go module supplemental license/notice

Source files: `go.uber.org/zap@v1.28.0 (go.uber.org/zap/LICENSE)`, `go.uber.org/zap@v1.28.0 (go.uber.org/zap/LICENSE; Windows desktop/installer)`

````text
Copyright (c) 2016-2024 Uber Technologies, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/array.go`, `go.uber.org/zap@v1.28.0/buffer/pool.go`, `go.uber.org/zap@v1.28.0/config.go`, `go.uber.org/zap@v1.28.0/encoder.go`, `go.uber.org/zap@v1.28.0/field.go`, `go.uber.org/zap@v1.28.0/flag.go`, `go.uber.org/zap@v1.28.0/global.go`, `go.uber.org/zap@v1.28.0/http_handler.go`, `go.uber.org/zap@v1.28.0/internal/ztest/timeout.go`, `go.uber.org/zap@v1.28.0/internal/ztest/writer.go`, `go.uber.org/zap@v1.28.0/level.go`, `go.uber.org/zap@v1.28.0/logger.go`, `go.uber.org/zap@v1.28.0/options.go`, `go.uber.org/zap@v1.28.0/sugar.go`, `go.uber.org/zap@v1.28.0/time.go`, `go.uber.org/zap@v1.28.0/zapcore/console_encoder.go`, `go.uber.org/zap@v1.28.0/zapcore/core.go`, `go.uber.org/zap@v1.28.0/zapcore/encoder.go`, `go.uber.org/zap@v1.28.0/zapcore/entry.go`, `go.uber.org/zap@v1.28.0/zapcore/field.go`, `go.uber.org/zap@v1.28.0/zapcore/hook.go`, `go.uber.org/zap@v1.28.0/zapcore/json_encoder.go`, `go.uber.org/zap@v1.28.0/zapcore/level.go`, `go.uber.org/zap@v1.28.0/zapcore/level_strings.go`, `go.uber.org/zap@v1.28.0/zapcore/marshaler.go`, `go.uber.org/zap@v1.28.0/zapcore/memory_encoder.go`, `go.uber.org/zap@v1.28.0/zapcore/reflected_encoder.go`, `go.uber.org/zap@v1.28.0/zapcore/write_syncer.go`, `go.uber.org/zap@v1.28.0/zaptest/timeout.go`, `go.uber.org/zap@v1.28.0/zaptest/writer.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/buffer/buffer.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package buffer provides a thin wrapper around a byte slice. Unlike the
// standard library's bytes.Buffer, it supports a portion of the strconv
// package's zero-allocation formatters.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/doc.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package zap provides fast, structured, leveled logging.
//
// For applications that log in the hot path, reflection-based serialization
// and string formatting are prohibitively expensive - they're CPU-intensive
// and make many small allocations. Put differently, using json.Marshal and
// fmt.Fprintf to log tons of interface{} makes your application slow.
//
// Zap takes a different approach. It includes a reflection-free,
// zero-allocation JSON encoder, and the base Logger strives to avoid
// serialization overhead and allocations wherever possible. By building the
// high-level SugaredLogger on that foundation, zap lets users choose when
// they need to count every allocation and when they'd prefer a more familiar,
// loosely typed API.
//
// # Choosing a Logger
//
// In contexts where performance is nice, but not critical, use the
// SugaredLogger. It's 4-10x faster than other structured logging packages and
// supports both structured and printf-style logging. Like log15 and go-kit,
// the SugaredLogger's structured logging APIs are loosely typed and accept a
// variadic number of key-value pairs. (For more advanced use cases, they also
// accept strongly typed fields - see the SugaredLogger.With documentation for
// details.)
//
//	sugar := zap.NewExample().Sugar()
//	defer sugar.Sync()
//	sugar.Infow("failed to fetch URL",
//	  "url", "http://example.com",
//	  "attempt", 3,
//	  "backoff", time.Second,
//	)
//	sugar.Infof("failed to fetch URL: %s", "http://example.com")
//
// By default, loggers are unbuffered. However, since zap's low-level APIs
// allow buffering, calling Sync before letting your process exit is a good
// habit.
//
// In the rare contexts where every microsecond and every allocation matter,
// use the Logger. It's even faster than the SugaredLogger and allocates far
// less, but it only supports strongly-typed, structured logging.
//
//	logger := zap.NewExample()
//	defer logger.Sync()
//	logger.Info("failed to fetch URL",
//	  zap.String("url", "http://example.com"),
//	  zap.Int("attempt", 3),
//	  zap.Duration("backoff", time.Second),
//	)
//
// Choosing between the Logger and SugaredLogger doesn't need to be an
// application-wide decision: converting between the two is simple and
// inexpensive.
//
//	logger := zap.NewExample()
//	defer logger.Sync()
//	sugar := logger.Sugar()
//	plain := sugar.Desugar()
//
// # Configuring Zap
//
// The simplest way to build a Logger is to use zap's opinionated presets:
// NewExample, NewProduction, and NewDevelopment. These presets build a logger
// with a single function call:
//
//	logger, err := zap.NewProduction()
//	if err != nil {
//	  log.Fatalf("can't initialize zap logger: %v", err)
//	}
//	defer logger.Sync()
//
// Presets are fine for small projects, but larger projects and organizations
// naturally require a bit more customization. For most users, zap's Config
// struct strikes the right balance between flexibility and convenience. See
// the package-level BasicConfiguration example for sample code.
//
// More unusual configurations (splitting output between files, sending logs
// to a message queue, etc.) are possible, but require direct use of
// go.uber.org/zap/zapcore. See the package-level AdvancedConfiguration
// example for sample code.
//
// # Extending Zap
//
// The zap package itself is a relatively thin wrapper around the interfaces
// in go.uber.org/zap/zapcore. Extending zap to support a new encoding (e.g.,
// BSON), a new log sink (e.g., Kafka), or something more exotic (perhaps an
// exception aggregation service, like Sentry or Rollbar) typically requires
// implementing the zapcore.Encoder, zapcore.WriteSyncer, or zapcore.Core
// interfaces. See the zapcore documentation for details.
//
// Similarly, package authors can use the high-performance Encoder and Core
// implementations in the zapcore package to build their own loggers.
//
// # Frequently Asked Questions
//
// An FAQ covering everything from installation errors to design decisions is
// available at https://github.com/uber-go/zap/blob/master/FAQ.md.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/error.go`, `go.uber.org/zap@v1.28.0/zapcore/error.go`, `go.uber.org/zap@v1.28.0/zaptest/logger.go`, `go.uber.org/zap@v1.28.0/zaptest/observer/logged_entry.go`, `go.uber.org/zap@v1.28.0/zaptest/testingt.go`

````text
// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/bufferpool/bufferpool.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package bufferpool houses zap's shared internal buffer pool. Third-party
// packages can recreate the same functionality with buffers.NewPool.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/color/color.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package color adds coloring functionality for TTY output.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/exit/exit.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package exit provides stubs so that unit tests can exercise code that calls
// os.Exit(1).
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/level_enabler.go`

````text
// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package internal and its subpackages hold types and functionality
// that are not part of Zap's public API.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/pool/pool.go`

````text
// Copyright (c) 2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package pool provides internal pool utilities.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/readme/readme.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// readme generates Zap's README from a template.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/stacktrace/stack.go`

````text
// Copyright (c) 2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package stacktrace provides support for gathering stack traces
// efficiently.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/ztest/clock.go`, `go.uber.org/zap@v1.28.0/zapcore/lazy_with.go`

````text
// Copyright (c) 2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/internal/ztest/doc.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package ztest provides low-level helpers for testing log output. These
// utilities are helpful in zap's own unit tests, but any assertions using
// them are strongly coupled to a single encoding.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/sink.go`, `go.uber.org/zap@v1.28.0/writer.go`, `go.uber.org/zap@v1.28.0/zapcore/sampler.go`, `go.uber.org/zap@v1.28.0/zapcore/tee.go`

````text
// Copyright (c) 2016-2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zapcore/buffered_write_syncer.go`, `go.uber.org/zap@v1.28.0/zapcore/clock.go`

````text
// Copyright (c) 2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zapcore/doc.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package zapcore defines and implements the low-level interfaces upon which
// zap is built. By providing alternate implementations of these interfaces,
// external packages can extend zap's capabilities.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zapcore/increase_level.go`

````text
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zapgrpc/zapgrpc.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package zapgrpc provides a logger that is compatible with grpclog.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zapio/writer.go`

````text
// Copyright (c) 2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package zapio provides tools for interacting with IO streams through Zap.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zaptest/doc.go`

````text
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package zaptest provides a variety of helpers for testing log output.
````

### Go module inline license

Source files: `go.uber.org/zap@v1.28.0/zaptest/observer/observer.go`

````text
// Copyright (c) 2016-2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package observer provides a zapcore.Core that keeps an in-memory,
// encoding-agnostic representation of log entries. It's useful for
// applications that want to unit test their log output without tying their
// tests to a particular output encoding.
````

### Go toolchain patent notice

Source files: `go1.27.4 GOROOT/PATENTS`

````text
Additional IP Rights Grant (Patents)

"This implementation" means the copyrightable works distributed by
Google as part of the Go project.

Google hereby grants to You a perpetual, worldwide, non-exclusive,
no-charge, royalty-free, irrevocable (except as stated in this section)
patent license to make, have made, use, offer to sell, sell, import,
transfer and otherwise run, modify and propagate the contents of this
implementation of Go, where such license applies only to those patent
claims, both currently owned or controlled by Google and acquired in
the future, licensable by Google that are necessarily infringed by this
implementation of Go.  This grant does not include claims that would be
infringed only as a consequence of further modification of this
implementation.  If you or your agent or exclusive licensee institute or
order or agree to the institution of patent litigation against any
entity (including a cross-claim or counterclaim in a lawsuit) alleging
that this implementation of Go or any code incorporated within this
implementation of Go constitutes direct or contributory patent
infringement, or inducement of patent infringement, then any patent
rights granted to you under this License for this implementation of Go
shall terminate as of the date such litigation is filed.
````

### BSD-3-Clause (Go Authors), Go module license, Go module supplemental license/notice, Go standard library vendored license

Source files: `go1.27.4 Go standard library/runtime (GOROOT/LICENSE)`, `go1.27.4 src/vendor/golang.org/x/crypto/LICENSE`, `go1.27.4 src/vendor/golang.org/x/net/LICENSE`, `go1.27.4 src/vendor/golang.org/x/sys/LICENSE`, `go1.27.4 src/vendor/golang.org/x/text/LICENSE`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; Android gomobile AAR)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; Linux WebUI arm64)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; Linux WebUI)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; Windows desktop/installer)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; macOS desktop arm64)`, `golang.org/x/crypto@v0.59.0 (golang.org/x/crypto/LICENSE; macOS desktop)`, `golang.org/x/mobile@v0.0.0-20261117202452-47699113848a (golang.org/x/mobile/LICENSE)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; Android gomobile AAR)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; Linux WebUI arm64)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; Linux WebUI)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; Windows desktop/installer)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; macOS desktop arm64)`, `golang.org/x/net@v0.63.0 (golang.org/x/net/LICENSE; macOS desktop)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/LICENSE)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/LICENSE; Linux WebUI)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/LICENSE; Windows desktop/installer)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/LICENSE; macOS desktop)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/unix/LICENSE; Android gomobile AAR)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/unix/LICENSE; Linux WebUI arm64)`, `golang.org/x/sys@v0.50.0 (golang.org/x/sys/unix/LICENSE; macOS desktop arm64)`, `golang.org/x/term@v0.48.0 (golang.org/x/term/LICENSE)`, `golang.org/x/term@v0.48.0 (golang.org/x/term/LICENSE; macOS desktop arm64)`, `golang.org/x/term@v0.48.0 (golang.org/x/term/LICENSE; macOS desktop)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; Android gomobile AAR)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; Linux WebUI arm64)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; Linux WebUI)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; Windows desktop/installer)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; macOS desktop arm64)`, `golang.org/x/text@v0.44.0 (golang.org/x/text/LICENSE; macOS desktop)`

````text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### Go standard library inline license

Source files: `go1.27.4 src/crypto/internal/fips140/edwards25519/scalar.go`

````text
// The field implementation in scalar_fiat.go is generated by the fiat-crypto
// project (https://github.com/mit-plv/fiat-crypto) at version v0.0.9 (23d2dbc)
// from a formally verified model.
//
// fiat-crypto code comes under the following license.
//
//     Copyright (c) 2015-2020 The fiat-crypto Authors. All rights reserved.
//
//     Redistribution and use in source and binary forms, with or without
//     modification, are permitted provided that the following conditions are
//     met:
//
//         1. Redistributions of source code must retain the above copyright
//         notice, this list of conditions and the following disclaimer.
//
//     THIS SOFTWARE IS PROVIDED BY the fiat-crypto authors "AS IS"
//     AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO,
//     THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
//     PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL Berkeley Software Design,
//     Inc. BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
//     EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
//     PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
//     PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
//     LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
//     NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
//     SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

// NewScalar returns a new zero Scalar.
````

### Go standard library inline license

Source files: `go1.27.4 src/crypto/internal/fips140/nistec/p256_ordinv.go`

````text
// The code below was generated by Fiat Cryptography v0.1.6-63-g92ee794c2, and
// then manually formatted and optimized.
//
//	word-by-word-montgomery --lang Go --no-wide-int
//	--relax-primitive-carry-to-bitwidth 32,64 --cmovznz-by-mul --static
//	--package-case flatcase --private-function-case camelCase --private-type-case
//	camelCase --no-prefix-fiat --package-name nistec p256Ord 64
//	2^256-2^224+2^192-89188191075325690597107910205041859247
//  mul square from_montgomery to_montgomery
//
// https://mit-plv.github.io/fiat-crypto/
//
//	 Copyright (c) 2015-2020 the fiat-crypto authors (see the AUTHORS file)
//	 All rights reserved.
//
//	 Redistribution and use in source and binary forms, with or without
//	 modification, are permitted provided that the following conditions are
//	 met:
//
//	 	1. Redistributions of source code must retain the above copyright
//	 	notice, this list of conditions and the following disclaimer.
//
//	 THIS SOFTWARE IS PROVIDED BY the fiat-crypto authors "AS IS"
//	 AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO,
//	 THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
//	 PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL Berkeley Software Design,
//	 Inc. BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
//	 EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
//	 PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
//	 PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
//	 LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
//	 NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
//	 SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

// p256OrdMontElement is a scalar field element in the Montgomery domain, as
// four uint64 limbs in little-endian order. It must be strictly less than
// ord(G) and in Montgomery form (with R 2²⁵⁶).
````

### Go standard library inline license

Source files: `go1.27.4 src/debug/elf/elf.go`

````text
/*
 * ELF constants and data structures
 *
 * Derived from:
 * $FreeBSD: src/sys/sys/elf32.h,v 1.8.14.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/sys/elf64.h,v 1.10.14.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/sys/elf_common.h,v 1.15.8.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/alpha/include/elf.h,v 1.14 2003/09/25 01:10:22 peter Exp $
 * $FreeBSD: src/sys/amd64/include/elf.h,v 1.18 2004/08/03 08:21:48 dfr Exp $
 * $FreeBSD: src/sys/arm/include/elf.h,v 1.5.2.1 2006/06/30 21:42:52 cognet Exp $
 * $FreeBSD: src/sys/i386/include/elf.h,v 1.16 2004/08/02 19:12:17 dfr Exp $
 * $FreeBSD: src/sys/powerpc/include/elf.h,v 1.7 2004/11/02 09:47:01 ssouhlal Exp $
 * $FreeBSD: src/sys/sparc64/include/elf.h,v 1.12 2003/09/25 01:10:26 peter Exp $
 * "System V ABI" (http://www.sco.com/developers/gabi/latest/ch4.eheader.html)
 * "ELF for the ARM® 64-bit Architecture (AArch64)" (ARM IHI 0056B)
 * "RISC-V ELF psABI specification" (https://github.com/riscv-non-isa/riscv-elf-psabi-doc/blob/master/riscv-elf.adoc)
 * llvm/BinaryFormat/ELF.h - ELF constants and structures
 *
 * Copyright (c) 1996-1998 John D. Polstra.  All rights reserved.
 * Copyright (c) 2001 David E. O'Brien
 * Portions Copyright 2009 The Go Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */
````

### Go standard library inline license

Source files: `go1.27.4 src/math/acosh.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_acosh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// __ieee754_acosh(x)
// Method :
//	Based on
//	        acosh(x) = log [ x + sqrt(x*x-1) ]
//	we have
//	        acosh(x) := log(x)+ln2,	if x is large; else
//	        acosh(x) := log(2x-1/(sqrt(x*x-1)+x)) if x>2; else
//	        acosh(x) := log1p(t+sqrt(2.0*t+t*t)); where t=x-1.
//
// Special cases:
//	acosh(x) is NaN with signal if x<1.
//	acosh(NaN) is NaN without signal.
//

// Acosh returns the inverse hyperbolic cosine of x.
//
// Special cases are:
//
//	Acosh(+Inf) = +Inf
//	Acosh(x) = NaN if x < 1
//	Acosh(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/asinh.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_asinh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// asinh(x)
// Method :
//	Based on
//	        asinh(x) = sign(x) * log [ |x| + sqrt(x*x+1) ]
//	we have
//	asinh(x) := x  if  1+x*x=1,
//	         := sign(x)*(log(x)+ln2) for large |x|, else
//	         := sign(x)*log(2|x|+1/(|x|+sqrt(x*x+1))) if|x|>2, else
//	         := sign(x)*log1p(|x| + x**2/(1 + sqrt(1+x**2)))
//

// Asinh returns the inverse hyperbolic sine of x.
//
// Special cases are:
//
//	Asinh(±0) = ±0
//	Asinh(±Inf) = ±Inf
//	Asinh(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/atanh.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_atanh.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// __ieee754_atanh(x)
// Method :
//	1. Reduce x to positive by atanh(-x) = -atanh(x)
//	2. For x>=0.5
//	            1              2x                          x
//	atanh(x) = --- * log(1 + -------) = 0.5 * log1p(2 * --------)
//	            2             1 - x                      1 - x
//
//	For x<0.5
//	atanh(x) = 0.5*log1p(2x+2x*x/(1-x))
//
// Special cases:
//	atanh(x) is NaN if |x| > 1 with signal;
//	atanh(NaN) is that NaN with no signal;
//	atanh(+-1) is +-INF with signal.
//

// Atanh returns the inverse hyperbolic tangent of x.
//
// Special cases are:
//
//	Atanh(1) = +Inf
//	Atanh(±0) = ±0
//	Atanh(-1) = -Inf
//	Atanh(x) = NaN if x < -1 or x > 1
//	Atanh(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/cbrt.go`

````text
// The go code is a modified version of the original C code from
// http://www.netlib.org/fdlibm/s_cbrt.c and came with this notice.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================

// Cbrt returns the cube root of x.
//
// Special cases are:
//
//	Cbrt(±0) = ±0
//	Cbrt(±Inf) = ±Inf
//	Cbrt(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/erf.go`

````text
/*
	Floating-point error function and complementary error function.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/s_erf.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// double erf(double x)
// double erfc(double x)
//                           x
//                    2      |\
//     erf(x)  =  ---------  | exp(-t*t)dt
//                 sqrt(pi) \|
//                           0
//
//     erfc(x) =  1-erf(x)
//  Note that
//              erf(-x) = -erf(x)
//              erfc(-x) = 2 - erfc(x)
//
// Method:
//      1. For |x| in [0, 0.84375]
//          erf(x)  = x + x*R(x**2)
//          erfc(x) = 1 - erf(x)           if x in [-.84375,0.25]
//                  = 0.5 + ((0.5-x)-x*R)  if x in [0.25,0.84375]
//         where R = P/Q where P is an odd poly of degree 8 and
//         Q is an odd poly of degree 10.
//                                               -57.90
//                      | R - (erf(x)-x)/x | <= 2
//
//
//         Remark. The formula is derived by noting
//          erf(x) = (2/sqrt(pi))*(x - x**3/3 + x**5/10 - x**7/42 + ....)
//         and that
//          2/sqrt(pi) = 1.128379167095512573896158903121545171688
//         is close to one. The interval is chosen because the fix
//         point of erf(x) is near 0.6174 (i.e., erf(x)=x when x is
//         near 0.6174), and by some experiment, 0.84375 is chosen to
//         guarantee the error is less than one ulp for erf.
//
//      2. For |x| in [0.84375,1.25], let s = |x| - 1, and
//         c = 0.84506291151 rounded to single (24 bits)
//              erf(x)  = sign(x) * (c  + P1(s)/Q1(s))
//              erfc(x) = (1-c)  - P1(s)/Q1(s) if x > 0
//                        1+(c+P1(s)/Q1(s))    if x < 0
//              |P1/Q1 - (erf(|x|)-c)| <= 2**-59.06
//         Remark: here we use the taylor series expansion at x=1.
//              erf(1+s) = erf(1) + s*Poly(s)
//                       = 0.845.. + P1(s)/Q1(s)
//         That is, we use rational approximation to approximate
//                      erf(1+s) - (c = (single)0.84506291151)
//         Note that |P1/Q1|< 0.078 for x in [0.84375,1.25]
//         where
//              P1(s) = degree 6 poly in s
//              Q1(s) = degree 6 poly in s
//
//      3. For x in [1.25,1/0.35(~2.857143)],
//              erfc(x) = (1/x)*exp(-x*x-0.5625+R1/S1)
//              erf(x)  = 1 - erfc(x)
//         where
//              R1(z) = degree 7 poly in z, (z=1/x**2)
//              S1(z) = degree 8 poly in z
//
//      4. For x in [1/0.35,28]
//              erfc(x) = (1/x)*exp(-x*x-0.5625+R2/S2) if x > 0
//                      = 2.0 - (1/x)*exp(-x*x-0.5625+R2/S2) if -6<x<0
//                      = 2.0 - tiny            (if x <= -6)
//              erf(x)  = sign(x)*(1.0 - erfc(x)) if x < 6, else
//              erf(x)  = sign(x)*(1.0 - tiny)
//         where
//              R2(z) = degree 6 poly in z, (z=1/x**2)
//              S2(z) = degree 7 poly in z
//
//      Note1:
//         To compute exp(-x*x-0.5625+R/S), let s be a single
//         precision number and s := x; then
//              -x*x = -s*s + (s-x)*(s+x)
//              exp(-x*x-0.5626+R/S) =
//                      exp(-s*s-0.5625)*exp((s-x)*(s+x)+R/S);
//      Note2:
//         Here 4 and 5 make use of the asymptotic series
//                        exp(-x*x)
//              erfc(x) ~ ---------- * ( 1 + Poly(1/x**2) )
//                        x*sqrt(pi)
//         We use rational approximation to approximate
//              g(s)=f(1/x**2) = log(erfc(x)*x) - x*x + 0.5625
//         Here is the error bound for R1/S1 and R2/S2
//              |R1/S1 - f(x)|  < 2**(-62.57)
//              |R2/S2 - f(x)|  < 2**(-61.52)
//
//      5. For inf > x >= 28
//              erf(x)  = sign(x) *(1 - tiny)  (raise inexact)
//              erfc(x) = tiny*tiny (raise underflow) if x > 0
//                      = 2 - tiny if x<0
//
//      7. Special case:
//              erf(0)  = 0, erf(inf)  = 1, erf(-inf) = -1,
//              erfc(0) = 1, erfc(inf) = 0, erfc(-inf) = 2,
//              erfc/erf(NaN) is NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/exp.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_exp.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved.
//
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// exp(x)
// Returns the exponential of x.
//
// Method
//   1. Argument reduction:
//      Reduce x to an r so that |r| <= 0.5*ln2 ~ 0.34658.
//      Given x, find r and integer k such that
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2.
//
//      Here r will be represented as r = hi-lo for better
//      accuracy.
//
//   2. Approximation of exp(r) by a special rational function on
//      the interval [0,0.34658]:
//      Write
//          R(r**2) = r*(exp(r)+1)/(exp(r)-1) = 2 + r*r/6 - r**4/360 + ...
//      We use a special Remez algorithm on [0,0.34658] to generate
//      a polynomial of degree 5 to approximate R. The maximum error
//      of this polynomial approximation is bounded by 2**-59. In
//      other words,
//          R(z) ~ 2.0 + P1*z + P2*z**2 + P3*z**3 + P4*z**4 + P5*z**5
//      (where z=r*r, and the values of P1 to P5 are listed below)
//      and
//          |                  5          |     -59
//          | 2.0+P1*z+...+P5*z   -  R(z) | <= 2
//          |                             |
//      The computation of exp(r) thus becomes
//                             2*r
//              exp(r) = 1 + -------
//                            R - r
//                                 r*R1(r)
//                     = 1 + r + ----------- (for better accuracy)
//                                2 - R1(r)
//      where
//                               2       4             10
//              R1(r) = r - (P1*r  + P2*r  + ... + P5*r   ).
//
//   3. Scale back to obtain exp(x):
//      From step 1, we have
//         exp(x) = 2**k * exp(r)
//
// Special cases:
//      exp(INF) is INF, exp(NaN) is NaN;
//      exp(-INF) is 0, and
//      for finite argument, only exp(0)=1 is exact.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Misc. info.
//      For IEEE double
//          if x >  7.09782712893383973096e+02 then exp(x) overflow
//          if x < -7.45133219101941108420e+02 then exp(x) underflow
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
````

### Go standard library inline license

Source files: `go1.27.4 src/math/expm1.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_expm1.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// expm1(x)
// Returns exp(x)-1, the exponential of x minus 1.
//
// Method
//   1. Argument reduction:
//      Given x, find r and integer k such that
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2 ~ 0.34658
//
//      Here a correction term c will be computed to compensate
//      the error in r when rounded to a floating-point number.
//
//   2. Approximating expm1(r) by a special rational function on
//      the interval [0,0.34658]:
//      Since
//          r*(exp(r)+1)/(exp(r)-1) = 2+ r**2/6 - r**4/360 + ...
//      we define R1(r*r) by
//          r*(exp(r)+1)/(exp(r)-1) = 2+ r**2/6 * R1(r*r)
//      That is,
//          R1(r**2) = 6/r *((exp(r)+1)/(exp(r)-1) - 2/r)
//                   = 6/r * ( 1 + 2.0*(1/(exp(r)-1) - 1/r))
//                   = 1 - r**2/60 + r**4/2520 - r**6/100800 + ...
//      We use a special Reme algorithm on [0,0.347] to generate
//      a polynomial of degree 5 in r*r to approximate R1. The
//      maximum error of this polynomial approximation is bounded
//      by 2**-61. In other words,
//          R1(z) ~ 1.0 + Q1*z + Q2*z**2 + Q3*z**3 + Q4*z**4 + Q5*z**5
//      where   Q1  =  -1.6666666666666567384E-2,
//              Q2  =   3.9682539681370365873E-4,
//              Q3  =  -9.9206344733435987357E-6,
//              Q4  =   2.5051361420808517002E-7,
//              Q5  =  -6.2843505682382617102E-9;
//      (where z=r*r, and the values of Q1 to Q5 are listed below)
//      with error bounded by
//          |                  5           |     -61
//          | 1.0+Q1*z+...+Q5*z   -  R1(z) | <= 2
//          |                              |
//
//      expm1(r) = exp(r)-1 is then computed by the following
//      specific way which minimize the accumulation rounding error:
//                             2     3
//                            r     r    [ 3 - (R1 + R1*r/2)  ]
//            expm1(r) = r + --- + --- * [--------------------]
//                            2     2    [ 6 - r*(3 - R1*r/2) ]
//
//      To compensate the error in the argument reduction, we use
//              expm1(r+c) = expm1(r) + c + expm1(r)*c
//                         ~ expm1(r) + c + r*c
//      Thus c+r*c will be added in as the correction terms for
//      expm1(r+c). Now rearrange the term to avoid optimization
//      screw up:
//                      (      2                                    2 )
//                      ({  ( r    [ R1 -  (3 - R1*r/2) ]  )  }    r  )
//       expm1(r+c)~r - ({r*(--- * [--------------------]-c)-c} - --- )
//                      ({  ( 2    [ 6 - r*(3 - R1*r/2) ]  )  }    2  )
//                      (                                             )
//
//                 = r - E
//   3. Scale back to obtain expm1(x):
//      From step 1, we have
//         expm1(x) = either 2**k*[expm1(r)+1] - 1
//                  = or     2**k*[expm1(r) + (1-2**-k)]
//   4. Implementation notes:
//      (A). To save one multiplication, we scale the coefficient Qi
//           to Qi*2**i, and replace z by (x**2)/2.
//      (B). To achieve maximum accuracy, we compute expm1(x) by
//        (i)   if x < -56*ln2, return -1.0, (raise inexact if x!=inf)
//        (ii)  if k=0, return r-E
//        (iii) if k=-1, return 0.5*(r-E)-0.5
//        (iv)  if k=1 if r < -0.25, return 2*((r+0.5)- E)
//                     else          return  1.0+2.0*(r-E);
//        (v)   if (k<-2||k>56) return 2**k(1-(E-r)) - 1 (or exp(x)-1)
//        (vi)  if k <= 20, return 2**k((1-2**-k)-(E-r)), else
//        (vii) return 2**k(1-((E+2**-k)-r))
//
// Special cases:
//      expm1(INF) is INF, expm1(NaN) is NaN;
//      expm1(-INF) is -1, and
//      for finite argument, only expm1(0)=0 is exact.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Misc. info.
//      For IEEE double
//          if x >  7.09782712893383973096e+02 then expm1(x) overflow
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
//

// Expm1 returns e**x - 1, the base-e exponential of x minus 1.
// It is more accurate than [Exp](x) - 1 when x is near zero.
//
// Special cases are:
//
//	Expm1(+Inf) = +Inf
//	Expm1(-Inf) = -1
//	Expm1(NaN) = NaN
//
// Very large values overflow to -1 or +Inf.
````

### Go standard library inline license

Source files: `go1.27.4 src/math/j0.go`

````text
/*
	Bessel function of the first and second kinds of order zero.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_j0.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_j0(x), __ieee754_y0(x)
// Bessel function of the first and second kinds of order zero.
// Method -- j0(x):
//      1. For tiny x, we use j0(x) = 1 - x**2/4 + x**4/64 - ...
//      2. Reduce x to |x| since j0(x)=j0(-x),  and
//         for x in (0,2)
//              j0(x) = 1-z/4+ z**2*R0/S0,  where z = x*x;
//         (precision:  |j0-1+z/4-z**2R0/S0 |<2**-63.67 )
//         for x in (2,inf)
//              j0(x) = sqrt(2/(pi*x))*(p0(x)*cos(x0)-q0(x)*sin(x0))
//         where x0 = x-pi/4. It is better to compute sin(x0),cos(x0)
//         as follow:
//              cos(x0) = cos(x)cos(pi/4)+sin(x)sin(pi/4)
//                      = 1/sqrt(2) * (cos(x) + sin(x))
//              sin(x0) = sin(x)cos(pi/4)-cos(x)sin(pi/4)
//                      = 1/sqrt(2) * (sin(x) - cos(x))
//         (To avoid cancellation, use
//              sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
//         to compute the worse one.)
//
//      3 Special cases
//              j0(nan)= nan
//              j0(0) = 1
//              j0(inf) = 0
//
// Method -- y0(x):
//      1. For x<2.
//         Since
//              y0(x) = 2/pi*(j0(x)*(ln(x/2)+Euler) + x**2/4 - ...)
//         therefore y0(x)-2/pi*j0(x)*ln(x) is an even function.
//         We use the following function to approximate y0,
//              y0(x) = U(z)/V(z) + (2/pi)*(j0(x)*ln(x)), z= x**2
//         where
//              U(z) = u00 + u01*z + ... + u06*z**6
//              V(z) = 1  + v01*z + ... + v04*z**4
//         with absolute approximation error bounded by 2**-72.
//         Note: For tiny x, U/V = u0 and j0(x)~1, hence
//              y0(tiny) = u0 + (2/pi)*ln(tiny), (choose tiny<2**-27)
//      2. For x>=2.
//              y0(x) = sqrt(2/(pi*x))*(p0(x)*cos(x0)+q0(x)*sin(x0))
//         where x0 = x-pi/4. It is better to compute sin(x0),cos(x0)
//         by the method mentioned above.
//      3. Special cases: y0(0)=-inf, y0(x<0)=NaN, y0(inf)=0.
//

// J0 returns the order-zero Bessel function of the first kind.
//
// Special cases are:
//
//	J0(±Inf) = 0
//	J0(0) = 1
//	J0(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/j1.go`

````text
/*
	Bessel function of the first and second kinds of order one.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_j1.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_j1(x), __ieee754_y1(x)
// Bessel function of the first and second kinds of order one.
// Method -- j1(x):
//      1. For tiny x, we use j1(x) = x/2 - x**3/16 + x**5/384 - ...
//      2. Reduce x to |x| since j1(x)=-j1(-x),  and
//         for x in (0,2)
//              j1(x) = x/2 + x*z*R0/S0,  where z = x*x;
//         (precision:  |j1/x - 1/2 - R0/S0 |<2**-61.51 )
//         for x in (2,inf)
//              j1(x) = sqrt(2/(pi*x))*(p1(x)*cos(x1)-q1(x)*sin(x1))
//              y1(x) = sqrt(2/(pi*x))*(p1(x)*sin(x1)+q1(x)*cos(x1))
//         where x1 = x-3*pi/4. It is better to compute sin(x1),cos(x1)
//         as follow:
//              cos(x1) =  cos(x)cos(3pi/4)+sin(x)sin(3pi/4)
//                      =  1/sqrt(2) * (sin(x) - cos(x))
//              sin(x1) =  sin(x)cos(3pi/4)-cos(x)sin(3pi/4)
//                      = -1/sqrt(2) * (sin(x) + cos(x))
//         (To avoid cancellation, use
//              sin(x) +- cos(x) = -cos(2x)/(sin(x) -+ cos(x))
//         to compute the worse one.)
//
//      3 Special cases
//              j1(nan)= nan
//              j1(0) = 0
//              j1(inf) = 0
//
// Method -- y1(x):
//      1. screen out x<=0 cases: y1(0)=-inf, y1(x<0)=NaN
//      2. For x<2.
//         Since
//              y1(x) = 2/pi*(j1(x)*(ln(x/2)+Euler)-1/x-x/2+5/64*x**3-...)
//         therefore y1(x)-2/pi*j1(x)*ln(x)-1/x is an odd function.
//         We use the following function to approximate y1,
//              y1(x) = x*U(z)/V(z) + (2/pi)*(j1(x)*ln(x)-1/x), z= x**2
//         where for x in [0,2] (abs err less than 2**-65.89)
//              U(z) = U0[0] + U0[1]*z + ... + U0[4]*z**4
//              V(z) = 1  + v0[0]*z + ... + v0[4]*z**5
//         Note: For tiny x, 1/x dominate y1 and hence
//              y1(tiny) = -2/pi/tiny, (choose tiny<2**-54)
//      3. For x>=2.
//               y1(x) = sqrt(2/(pi*x))*(p1(x)*sin(x1)+q1(x)*cos(x1))
//         where x1 = x-3*pi/4. It is better to compute sin(x1),cos(x1)
//         by method mentioned above.

// J1 returns the order-one Bessel function of the first kind.
//
// Special cases are:
//
//	J1(±Inf) = 0
//	J1(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/jn.go`

````text
/*
	Bessel function of the first and second kinds of order n.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_jn.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_jn(n, x), __ieee754_yn(n, x)
// floating point Bessel's function of the 1st and 2nd kind
// of order n
//
// Special cases:
//      y0(0)=y1(0)=yn(n,0) = -inf with division by zero signal;
//      y0(-ve)=y1(-ve)=yn(n,-ve) are NaN with invalid signal.
// Note 2. About jn(n,x), yn(n,x)
//      For n=0, j0(x) is called,
//      for n=1, j1(x) is called,
//      for n<x, forward recursion is used starting
//      from values of j0(x) and j1(x).
//      for n>x, a continued fraction approximation to
//      j(n,x)/j(n-1,x) is evaluated and then backward
//      recursion is used starting from a supposed value
//      for j(n,x). The resulting value of j(0,x) is
//      compared with the actual value to correct the
//      supposed value of j(n,x).
//
//      yn(n,x) is similar in all respects, except
//      that forward recursion is used for all
//      values of n>1.

// Jn returns the order-n Bessel function of the first kind.
//
// Special cases are:
//
//	Jn(n, ±Inf) = 0
//	Jn(n, NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/lgamma.go`

````text
/*
	Floating-point logarithm of the Gamma function.
*/

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_lgamma_r.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_lgamma_r(x, signgamp)
// Reentrant version of the logarithm of the Gamma function
// with user provided pointer for the sign of Gamma(x).
//
// Method:
//   1. Argument Reduction for 0 < x <= 8
//      Since gamma(1+s)=s*gamma(s), for x in [0,8], we may
//      reduce x to a number in [1.5,2.5] by
//              lgamma(1+s) = log(s) + lgamma(s)
//      for example,
//              lgamma(7.3) = log(6.3) + lgamma(6.3)
//                          = log(6.3*5.3) + lgamma(5.3)
//                          = log(6.3*5.3*4.3*3.3*2.3) + lgamma(2.3)
//   2. Polynomial approximation of lgamma around its
//      minimum (ymin=1.461632144968362245) to maintain monotonicity.
//      On [ymin-0.23, ymin+0.27] (i.e., [1.23164,1.73163]), use
//              Let z = x-ymin;
//              lgamma(x) = -1.214862905358496078218 + z**2*poly(z)
//              poly(z) is a 14 degree polynomial.
//   2. Rational approximation in the primary interval [2,3]
//      We use the following approximation:
//              s = x-2.0;
//              lgamma(x) = 0.5*s + s*P(s)/Q(s)
//      with accuracy
//              |P/Q - (lgamma(x)-0.5s)| < 2**-61.71
//      Our algorithms are based on the following observation
//
//                             zeta(2)-1    2    zeta(3)-1    3
// lgamma(2+s) = s*(1-Euler) + --------- * s  -  --------- * s  + ...
//                                 2                 3
//
//      where Euler = 0.5772156649... is the Euler constant, which
//      is very close to 0.5.
//
//   3. For x>=8, we have
//      lgamma(x)~(x-0.5)log(x)-x+0.5*log(2pi)+1/(12x)-1/(360x**3)+....
//      (better formula:
//         lgamma(x)~(x-0.5)*(log(x)-1)-.5*(log(2pi)-1) + ...)
//      Let z = 1/x, then we approximation
//              f(z) = lgamma(x) - (x-0.5)(log(x)-1)
//      by
//                                  3       5             11
//              w = w0 + w1*z + w2*z  + w3*z  + ... + w6*z
//      where
//              |w - f(z)| < 2**-58.74
//
//   4. For negative x, since (G is gamma function)
//              -x*G(-x)*G(x) = pi/sin(pi*x),
//      we have
//              G(x) = pi/(sin(pi*x)*(-x)*G(-x))
//      since G(-x) is positive, sign(G(x)) = sign(sin(pi*x)) for x<0
//      Hence, for x<0, signgam = sign(sin(pi*x)) and
//              lgamma(x) = log(|Gamma(x)|)
//                        = log(pi/(|x*sin(pi*x)|)) - lgamma(-x);
//      Note: one should avoid computing pi*(-x) directly in the
//            computation of sin(pi*(-x)).
//
//   5. Special Cases
//              lgamma(2+s) ~ s*(1-Euler) for tiny s
//              lgamma(1)=lgamma(2)=0
//              lgamma(x) ~ -log(x) for tiny x
//              lgamma(0) = lgamma(inf) = inf
//              lgamma(-integer) = +-inf
//
//
````

### Go standard library inline license

Source files: `go1.27.4 src/math/log.go`

````text
/*
	Floating-point logarithm.
*/

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_log.c
// and came with this notice. The go code is a simpler
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_log(x)
// Return the logarithm of x
//
// Method :
//   1. Argument Reduction: find k and f such that
//			x = 2**k * (1+f),
//	   where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//   2. Approximation of log(1+f).
//	Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//		 = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//	     	 = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//	a polynomial of degree 14 to approximate R.  The maximum error
//	of this polynomial approximation is bounded by 2**-58.45. In
//	other words,
//		        2      4      6      8      10      12      14
//	    R(z) ~ L1*s +L2*s +L3*s +L4*s +L5*s  +L6*s  +L7*s
//	(the values of L1 to L7 are listed in the program) and
//	    |      2          14          |     -58.45
//	    | L1*s +...+L7*s    -  R(z) | <= 2
//	    |                             |
//	Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//	In order to guarantee error in log below 1ulp, we compute log by
//		log(1+f) = f - s*(f - R)		(if f is not too large)
//		log(1+f) = f - (hfsq - s*(hfsq+R)).	(better accuracy)
//
//	3. Finally,  log(x) = k*Ln2 + log(1+f).
//			    = k*Ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*Ln2_lo)))
//	   Here Ln2 is split into two floating point number:
//			Ln2_hi + Ln2_lo,
//	   where n*Ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//	log(x) is NaN with signal if x < 0 (including -INF) ;
//	log(+INF) is +INF; log(0) is -INF with signal;
//	log(NaN) is that NaN with no signal.
//
// Accuracy:
//	according to an error analysis, the error is always less than
//	1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.

// Log returns the natural logarithm of x.
//
// Special cases are:
//
//	Log(+Inf) = +Inf
//	Log(0) = -Inf
//	Log(x < 0) = NaN
//	Log(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/log1p.go`

````text
// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_log1p.c
// and came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// double log1p(double x)
//
// Method :
//   1. Argument Reduction: find k and f such that
//                      1+x = 2**k * (1+f),
//         where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//      Note. If k=0, then f=x is exact. However, if k!=0, then f
//      may not be representable exactly. In that case, a correction
//      term is need. Let u=1+x rounded. Let c = (1+x)-u, then
//      log(1+x) - log(u) ~ c/u. Thus, we proceed to compute log(u),
//      and add back the correction term c/u.
//      (Note: when x > 2**53, one can simply return log(x))
//
//   2. Approximation of log1p(f).
//      Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//               = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//               = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//      a polynomial of degree 14 to approximate R The maximum error
//      of this polynomial approximation is bounded by 2**-58.45. In
//      other words,
//                      2      4      6      8      10      12      14
//          R(z) ~ Lp1*s +Lp2*s +Lp3*s +Lp4*s +Lp5*s  +Lp6*s  +Lp7*s
//      (the values of Lp1 to Lp7 are listed in the program)
//      and
//          |      2          14          |     -58.45
//          | Lp1*s +...+Lp7*s    -  R(z) | <= 2
//          |                             |
//      Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//      In order to guarantee error in log below 1ulp, we compute log
//      by
//              log1p(f) = f - (hfsq - s*(hfsq+R)).
//
//   3. Finally, log1p(x) = k*ln2 + log1p(f).
//                        = k*ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*ln2_lo)))
//      Here ln2 is split into two floating point number:
//                   ln2_hi + ln2_lo,
//      where n*ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//      log1p(x) is NaN with signal if x < -1 (including -INF) ;
//      log1p(+INF) is +INF; log1p(-1) is -INF with signal;
//      log1p(NaN) is that NaN with no signal.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
//
// Note: Assuming log() return accurate answer, the following
//       algorithm can be used to compute log1p(x) to within a few ULP:
//
//              u = 1+x;
//              if(u==1.0) return x ; else
//                         return log(u)*(x/(u-1.0));
//
//       See HP-15C Advanced Functions Handbook, p.193.

// Log1p returns the natural logarithm of 1 plus its argument x.
// It is more accurate than [Log](1 + x) when x is near zero.
//
// Special cases are:
//
//	Log1p(+Inf) = +Inf
//	Log1p(±0) = ±0
//	Log1p(-1) = -Inf
//	Log1p(x < -1) = NaN
//	Log1p(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/remainder.go`

````text
// The original C code and the comment below are from
// FreeBSD's /usr/src/lib/msun/src/e_remainder.c and came
// with this notice. The go code is a simplified version of
// the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_remainder(x,y)
// Return :
//      returns  x REM y  =  x - [x/y]*y  as if in infinite
//      precision arithmetic, where [x/y] is the (infinite bit)
//      integer nearest x/y (in half way cases, choose the even one).
// Method :
//      Based on Mod() returning  x - [x/y]chopped * y  exactly.

// Remainder returns the IEEE 754 floating-point remainder of x/y.
//
// Special cases are:
//
//	Remainder(±Inf, y) = NaN
//	Remainder(NaN, y) = NaN
//	Remainder(x, 0) = NaN
//	Remainder(x, ±Inf) = x
//	Remainder(x, NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/math/sqrt.go`

````text
// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_sqrt.c and
// came with this notice. The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_sqrt(x)
// Return correctly rounded sqrt.
//           -----------------------------------------
//           | Use the hardware sqrt if you have one |
//           -----------------------------------------
// Method:
//   Bit by bit method using integer arithmetic. (Slow, but portable)
//   1. Normalization
//      Scale x to y in [1,4) with even powers of 2:
//      find an integer k such that  1 <= (y=x*2**(2k)) < 4, then
//              sqrt(x) = 2**k * sqrt(y)
//   2. Bit by bit computation
//      Let q  = sqrt(y) truncated to i bit after binary point (q = 1),
//           i                                                   0
//                                     i+1         2
//          s  = 2*q , and      y  =  2   * ( y - q  ).          (1)
//           i      i            i                 i
//
//      To compute q    from q , one checks whether
//                  i+1       i
//
//                            -(i+1) 2
//                      (q + 2      )  <= y.                     (2)
//                        i
//                                                            -(i+1)
//      If (2) is false, then q   = q ; otherwise q   = q  + 2      .
//                             i+1   i             i+1   i
//
//      With some algebraic manipulation, it is not difficult to see
//      that (2) is equivalent to
//                             -(i+1)
//                      s  +  2       <= y                       (3)
//                       i                i
//
//      The advantage of (3) is that s  and y  can be computed by
//                                    i      i
//      the following recurrence formula:
//          if (3) is false
//
//          s     =  s  ,       y    = y   ;                     (4)
//           i+1      i          i+1    i
//
//      otherwise,
//                         -i                      -(i+1)
//          s     =  s  + 2  ,  y    = y  -  s  - 2              (5)
//           i+1      i          i+1    i     i
//
//      One may easily use induction to prove (4) and (5).
//      Note. Since the left hand side of (3) contain only i+2 bits,
//            it is not necessary to do a full (53-bit) comparison
//            in (3).
//   3. Final rounding
//      After generating the 53 bits result, we compute one more bit.
//      Together with the remainder, we can decide whether the
//      result is exact, bigger than 1/2ulp, or less than 1/2ulp
//      (it will never equal to 1/2ulp).
//      The rounding mode can be detected by checking whether
//      huge + tiny is equal to huge, and whether huge - tiny is
//      equal to huge for some floating point number "huge" and "tiny".
//
//
// Notes:  Rounding mode detection omitted. The constants "mask", "shift",
// and "bias" are found in src/math/bits.go

// Sqrt returns the square root of x.
//
// Special cases are:
//
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = ±0
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memclr_arm.s`

````text
// Inferno's libkern/memset-arm.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memset-arm.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memmove_386.s`

````text
// Inferno's libkern/memmove-386.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memmove-386.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build !plan9
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memmove_amd64.s`

````text
// Derived from Inferno's libkern/memmove-386.s (adapted for amd64)
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memmove-386.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build !plan9
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memmove_arm.s`

````text
// Inferno's libkern/memmove-arm.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memmove-arm.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memmove_plan9_386.s`

````text
// Inferno's libkern/memmove-386.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memmove-386.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/memmove_plan9_amd64.s`

````text
// Derived from Inferno's libkern/memmove-386.s (adapted for amd64)
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/memmove-386.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/vlop_386.s`

````text
// Inferno's libkern/vlop-386.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/vlop-386.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/vlop_arm.s`

````text
// Inferno's libkern/vlop-arm.s
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/vlop-arm.s
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
````

### Go standard library inline license

Source files: `go1.27.4 src/runtime/vlrt.go`

````text
// Inferno's libkern/vlrt-arm.c
// https://bitbucket.org/inferno-os/inferno-os/src/master/libkern/vlrt-arm.c
//
//         Copyright © 1994-1999 Lucent Technologies Inc. All rights reserved.
//         Revisions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com).  All rights reserved.
//         Portions Copyright 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:build arm || 386 || mips || mipsle
````

### Go module inline license

Source files: `golang.org/x/mobile@v0.0.0-20261117202452-47699113848a/example/network/main.go`

````text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux || windows

// An app that paints green if golang.org is reachable when the app first
// starts, or red otherwise.
//
// In order to access the network from the Android app, its AndroidManifest.xml
// file must include the permission to access the network.
//
//	http://developer.android.com/guide/topics/manifest/manifest-intro.html#perms
//
// The gomobile tool auto-generates a default AndroidManifest file by default
// unless the package directory contains the AndroidManifest.xml. Users can
// customize app behavior, such as permissions and app name, by providing
// the AndroidManifest file. This is irrelevant to iOS.
//
// Note: This demo is an early preview of Go 1.5. In order to build this
// program as an Android APK using the gomobile tool.
//
// See http://godoc.org/golang.org/x/mobile/cmd/gomobile to install gomobile.
//
// Get the network example and use gomobile to build or install it on your device.
//
//	$ go get -d golang.org/x/mobile/example/network
//	$ gomobile build golang.org/x/mobile/example/network # will build an APK
//
//	# plug your Android device to your computer or start an Android emulator.
//	# if you have adb installed on your machine, use gomobile install to
//	# build and deploy the APK to an Android target.
//	$ gomobile install golang.org/x/mobile/example/network
//
// Switch to your device or emulator to start the network application from
// the launcher.
// You can also run the application on your desktop by running the command
// below. (Note: It currently doesn't work on Windows.)
//
//	$ go install golang.org/x/mobile/example/network && network
````

### Go module inline license

Source files: `golang.org/x/mobile@v0.0.0-20261117202452-47699113848a/gl/consts.go`

````text
/*
Partially generated from the Khronos OpenGL API specification in XML
format, which is covered by the license:

	Copyright (c) 2013-2014 The Khronos Group Inc.

	Permission is hereby granted, free of charge, to any person obtaining a
	copy of this software and/or associated documentation files (the
	"Materials"), to deal in the Materials without restriction, including
	without limitation the rights to use, copy, modify, merge, publish,
	distribute, sublicense, and/or sell copies of the Materials, and to
	permit persons to whom the Materials are furnished to do so, subject to
	the following conditions:

	The above copyright notice and this permission notice shall be included
	in all copies or substantial portions of the Materials.

	THE MATERIALS ARE PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
	EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
	MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
	CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
	MATERIALS OR THE USE OR OTHER DEALINGS IN THE MATERIALS.

*/
````

### MIT

Source files: `hey-listen@1.0.8 (LICENSE)`

````text
MIT License

Copyright (c) 2018 Popmotion

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `magic-string@0.30.21 (LICENSE)`

````text
Copyright 2018 Rich Harris

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### MIT

Source files: `motion-dom@14.14.0 (LICENSE.md)`, `motion-utils@14.7.0 (LICENSE.md)`

````text
The MIT License (MIT)

Copyright (c) 2024 [Motion](https://motion.dev) B.V.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `motion-v@2.13.0 (LICENSE)`

````text
MIT License

Copyright (c) 2024 Rick Huang

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### MIT

Source files: `nanoid@3.3.20 (LICENSE)`

````text
The MIT License (MIT)

Copyright 2017 Andrey Sitnik <andrey@sitnik.ru>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### ISC

Source files: `picocolors@1.1.1 (LICENSE)`

````text
ISC License

Copyright (c) 2021-2024 Oleksii Raspopov, Kostiantyn Denysov, Anton Verinov

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
````

### MIT

Source files: `postcss@8.5.32 (LICENSE)`

````text
The MIT License (MIT)

Copyright 2013 Andrey Sitnik <andrey@sitnik.es>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
````

### BSD-3-Clause

Source files: `source-map-js@1.2.2 (LICENSE)`

````text

Copyright (c) 2009-2011, Mozilla Foundation and contributors
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the names of the Mozilla Foundation nor the names of project
  contributors may be used to endorse or promote products derived from this
  software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
````

### MIT

Source files: `tailwindcss@4.3.5 (LICENSE)`

````text
MIT License

Copyright (c) Tailwind Labs, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
````

### 0BSD

Source files: `tslib@2.8.1 (LICENSE.txt)`

````text
Copyright (c) Microsoft Corporation.

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.
````

<!-- markdownlint-enable MD010 -->
