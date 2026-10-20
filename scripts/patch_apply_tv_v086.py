from pathlib import Path

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

# 3) Replace the helper's fragile _onKey bounds with a balanced-brace scanner.
start_token = 'key_start = s.find("  KeyEventResult _onKey(FocusNode node, KeyEvent event) {")\n'
block_start = s.find(start_token)
block_end = s.find('\nold = """    final scaffold = Scaffold(', block_start)
if block_start < 0 or block_end < 0:
    raise SystemExit('helper _onKey block not found')
block = s[block_start:block_end]
new_key_start = block.find("new_key = r'''  KeyEventResult _onKey")
if new_key_start < 0:
    raise SystemExit('helper new_key literal not found')
literal_end = block.find("\n'''\n", new_key_start)
if literal_end < 0:
    raise SystemExit('helper new_key literal end not found')
new_key_literal = block[new_key_start:literal_end + 5]
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
''' + new_key_literal + '''s = s[:key_start] + new_key + s[key_end:]
'''
s = s[:block_start] + replacement + s[block_end:]

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
