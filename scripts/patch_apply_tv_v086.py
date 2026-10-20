from pathlib import Path
import re

p = Path('scripts/apply_tv_v086.py')
s = p.read_text()

# 1) Episode branch: comments sit between currentIndex and episodes in the real source.
old = '''needle = """    final currentIndex = _episodeIndex.clamp(0, detail.episodes.length - 1);\\n    final episodes = detail.episodes;\\n"""
branch = """    final currentIndex = _episodeIndex.clamp(0, detail.episodes.length - 1);\\n    final episodes = detail.episodes;\\n    if (_television) {\\n      final chosen = await showTvEpisodePanel(\\n        context,\\n        title: widget.drama.title,\\n        episodes: episodes,\\n        currentIndex: currentIndex,\\n      );\\n      if (chosen != null && mounted && !_closed) {\\n        await _playEpisode(chosen);\\n      }\\n      return;\\n    }\\n"""
if needle not in s:
    raise SystemExit("episode branch anchor missing")
s = s.replace(needle, branch, 1)
'''
new = '''needle = "    final episodes = detail.episodes;\\n"
branch = """    final episodes = detail.episodes;\\n    if (_television) {\\n      final chosen = await showTvEpisodePanel(\\n        context,\\n        title: widget.drama.title,\\n        episodes: episodes,\\n        currentIndex: currentIndex,\\n      );\\n      if (chosen != null && mounted && !_closed) {\\n        await _playEpisode(chosen);\\n      }\\n      return;\\n    }\\n"""
if needle not in s:
    raise SystemExit("episode branch anchor missing")
s = s.replace(needle, branch, 1)
'''
if old not in s:
    raise SystemExit('helper episode block not found')
s = s.replace(old, new, 1)

# 2) _FeedRemoteActions construction uses eight-space property indentation in the real file.
s = s.replace(
    'old = """      openEpisodes: () => unawaited(_openEpisodes()),\\n      holdStart: _holdStartForRemote,\\n"""',
    'old = """        openEpisodes: () => unawaited(_openEpisodes()),\\n        holdStart: _holdStartForRemote,\\n"""',
    1,
)
s = s.replace(
    'new = """      openEpisodes: () => unawaited(_openEpisodes()),\\n      openMenu: () => unawaited(_openTvPlayerMenu()),\\n      consumeBack: _consumeBackForRemote,\\n      holdStart: _holdStartForRemote,\\n"""',
    'new = """        openEpisodes: () => unawaited(_openEpisodes()),\\n        openMenu: () => unawaited(_openTvPlayerMenu()),\\n        consumeBack: _consumeBackForRemote,\\n        holdStart: _holdStartForRemote,\\n"""',
    1,
)

# 3) Replace the whole _onKey method by balanced braces instead of assuming the next method name.
pattern = re.compile(
    r'''key_start = s\.find\("  KeyEventResult _onKey\(FocusNode node, KeyEvent event\) \{"\)\nkey_end = s\.find\("\\n  Future<bool> _confirmLeavePlayback\(\)", key_start\)\nif key_start < 0 or key_end < 0:\n    raise SystemExit\("parent _onKey block bounds missing"\)\n(new_key = r\'''[\s\S]*?\'''\n)s = s\[:key_start\] \+ new_key \+ s\[key_end:\]'''
)
match = pattern.search(s)
if not match:
    raise SystemExit('helper _onKey replacement block not found')
replacement = '''key_start = s.find("  KeyEventResult _onKey(FocusNode node, KeyEvent event) {")
if key_start < 0:
    raise SystemExit("parent _onKey start missing")
brace_start = s.find("{", key_start)
depth = 0
key_end = -1
for index in range(brace_start, len(s)):
    if s[index] == "{":
        depth += 1
    elif s[index] == "}":
        depth -= 1
        if depth == 0:
            key_end = index + 1
            break
if key_end < 0:
    raise SystemExit("parent _onKey end missing")
''' + match.group(1) + '''s = s[:key_start] + new_key + s[key_end:]
'''
s = s[:match.start()] + replacement + s[match.end():]

# 4) Go raw strings must contain JSON quotes directly, not backslash-escaped quotes.
s = s.replace(
    r'''`{\"version\":1,\"ttlSeconds\":3600,\"sources\":{\"huangdou\":[\"https://a.example\",\"https://a.example/\",\"http://bad.example\",\"https://b.example/path\"]}}`''',
    r'''`{"version":1,"ttlSeconds":3600,"sources":{"huangdou":["https://a.example","https://a.example/","http://bad.example","https://b.example/path"]}}`''',
)
s = s.replace(
    r'''`{\"version\":9,\"sources\":{\"huangdou\":[\"https://a.example\"]}}`''',
    r'''`{"version":9,"sources":{"huangdou":["https://a.example"]}}`''',
)

p.write_text(s)
