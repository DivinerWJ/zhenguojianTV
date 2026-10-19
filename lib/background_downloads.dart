/// Clean TV build: Android foreground-service notifications are disabled.
/// Downloads/media work stays in the app process and never requests notification permission.
class BackgroundDownloads {
  static Future<void> prepare() async {}
  static Future<void> ensureStarted() async {}
}
