from pathlib import Path

p = Path('scripts/apply_tv_v086.py')
s = p.read_text()
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
p.write_text(s.replace(old, new, 1))
