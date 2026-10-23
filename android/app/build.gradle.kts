import java.util.Properties
import java.util.Base64

plugins {
    id("com.android.application")
    id("dev.flutter.flutter-gradle-plugin")
}

val dartDefines = providers.gradleProperty("dart-defines").orNull.orEmpty()
    .split(",").filter { it.isNotEmpty() }
    .associate {
        val decoded = String(Base64.getDecoder().decode(it), Charsets.UTF_8)
        decoded.substringBefore("=") to decoded.substringAfter("=", "")
    }
val allSources = dartDefines["ALL_SOURCES"] == "true"

// 0.8.10 第三批：通用（all）APK 的 ABI 白名单。
//
// 用户反馈 all 版装到手机上提示"解析软件包失败"，而按 ABI 分包的两个包正常。
// 排查 all 包发现里面留着一个**半套**的 `lib/x86_64/`：ffmpeg / media-kit 这些
// 依赖的 AAR 自带 x86_64 的 .so，但 app 自身的 libapp.so / libflutter.so /
// libduanju_core.so 只按 ARM 两套编译，于是 x86_64 目录里缺了最关键的那几个。
// 这种"声明了某 ABI 却给不全"的包既白占十几 MB，也会让部分安装器按错误的
// ABI 去校验原生库，直接判解析失败。
//
// 构建通用包时由 scripts/build_android.py 通过
// `ORG_GRADLE_PROJECT_universalAbiFilters=arm64-v8a,armeabi-v7a` 注入白名单，
// 只打 ARM 两套；按 ABI 分包的构建不设这个属性，行为与原来完全一致。
val universalAbiFilters = providers.gradleProperty("universalAbiFilters").orNull
    ?.split(",")?.map { it.trim() }?.filter { it.isNotEmpty() }?.takeIf { it.isNotEmpty() }

// 0.8.10 第三批：通用（all）包的 versionCode 抬高。
//
// Flutter 的 `--split-per-abi` 会给分包加 1000/2000/3000 的 versionCode 偏移
// （arm64-v8a 是 2085、armeabi-v7a 是 1085，通用包却还是 85）。用户先装过
// 分包、再装 all 包时，安装器看到的是**版本降级**，很多 ROM 直接弹
// "解析软件包失败"。这里给通用包加一个偏移，让它永远比分包高。
val universalVersionCodeOffset = providers.gradleProperty("universalVersionCodeOffset").orNull?.toIntOrNull()

val releaseKey = rootProject.file("key.properties")
// CI 生成并缓存的固定调试签名（见 .github/workflows/build.yml）。
val ciDebugKey = rootProject.file("app/ci-debug.keystore")
val releaseProperties = Properties()
if (releaseKey.exists()) {
    releaseKey.inputStream().use { releaseProperties.load(it) }
}

android {
    namespace = "com.duanju.duanju_app"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = "28.2.13676358"

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "com.duanju.duanju_app"
        // 0.8.3：26 → 24。
        //
        // 电视盒子换代很慢，Android 7.0 / 7.1（API 24 / 25）的机器
        // （运营商送的魔百和、老款小米盒子、部分投影仪）存量很大，
        // minSdk 26 会直接把这些设备挡在安装门外（INSTALL_FAILED_OLDER_SDK）。
        //
        // 为什么是 24 而不是更低：ffmpeg_kit_flutter_new_min_gpl 的 AAR 声明
        // minSdkVersion 24，再往下会触发 manifest merger 失败。24 已能覆盖 7.x，
        // 5.x / 6.x 机器（API 21-23）仍不支持，需要换 FFmpeg 依赖才能再降。
        minSdk = 24
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode + (universalVersionCodeOffset ?: 0)
        versionName = flutter.versionName
        manifestPlaceholders["appLabel"] = if (allSources) "真果鉴TV" else "红果鉴"
        manifestPlaceholders["appBanner"] = if (allSources) "@drawable/tv_banner_all_sources" else "@drawable/tv_banner"
        // 通用包只保留 ARM 两套 ABI，见文件上方 universalAbiFilters 的说明。
        universalAbiFilters?.let { filters ->
            ndk {
                abiFilters.clear()
                abiFilters.addAll(filters)
            }
        }
    }

    signingConfigs {
        if (releaseKey.exists()) {
            create("release") {
                storeFile = file(requireNotNull(releaseProperties.getProperty("storeFile")))
                storePassword = requireNotNull(releaseProperties.getProperty("storePassword"))
                keyAlias = requireNotNull(releaseProperties.getProperty("keyAlias"))
                keyPassword = requireNotNull(releaseProperties.getProperty("keyPassword"))
            }
        }
        // 0.8.10 第三批：CI 用**固定**的调试签名。
        //
        // 没有 key.properties 时 AGP 用 `~/.android/debug.keystore`，而这个文件在
        // 每台 runner 上都是新生成的随机密钥 —— 于是每次构建签名都不一样，
        // 用户装新包时安装器判签名冲突（部分 ROM 报的就是"解析软件包失败"），
        // 每次升级都得先卸载。工作流会在构建前准备好 ci-debug.keystore 并缓存，
        // 这里优先用它；本地开发没有这个文件时仍然回落到默认 debug 签名。
        if (!releaseKey.exists() && ciDebugKey.exists()) {
            create("ciDebug") {
                storeFile = ciDebugKey
                storePassword = "android"
                keyAlias = "androiddebugkey"
                keyPassword = "android"
            }
        }
        // 0.8.10 第三批：显式打开 v1(JAR) 兼容签名。
        // minSdk = 24 时 AGP 默认只出 v2；只读 META-INF 证书的老安装器会判解析失败。
        all {
            enableV1Signing = true
            enableV2Signing = true
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
        }
        release {
            signingConfig = signingConfigs.getByName(
                when {
                    releaseKey.exists() -> "release"
                    ciDebugKey.exists() -> "ciDebug"
                    else -> "debug"
                })
        }
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter { source = "../.." }

tasks.withType<JavaCompile>().configureEach {
    if (name.contains("Release")) {
        doFirst {
            val registrant = file("src/main/java/io/flutter/plugins/GeneratedPluginRegistrant.java")
            if (registrant.exists()) {
                val generated = registrant.readText()
                val integrationPlugin = Regex(
                    """(?s)    try \{\s*flutterEngine\.getPlugins\(\)\.add\(new dev\.flutter\.plugins\.integration_test\.IntegrationTestPlugin\(\)\);\s*\} catch \(Exception e\) \{[^}]*\}\s*"""
                )
                val release = generated.replace(integrationPlugin, "")
                if (release != generated) registrant.writeText(release)
            }
        }
    }
}
