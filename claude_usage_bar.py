#!/usr/bin/env python3
"""Claude Usage System Tray App — macOS and Ubuntu compatible."""

import json
import os
import threading
import subprocess
import platform
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path

if platform.system() == 'Linux':
    _local_gi = str(Path.home() / '.local' / 'share' / 'girepository-1.0')
    if _local_gi not in os.environ.get('GI_TYPELIB_PATH', ''):
        os.environ['GI_TYPELIB_PATH'] = _local_gi + ':' + os.environ.get('GI_TYPELIB_PATH', '')

    if 'PYSTRAY_BACKEND' not in os.environ:
        try:
            import gi
            gi.require_version('AyatanaAppIndicator3', '0.1')
        except (ImportError, ValueError):
            try:
                import gi
                gi.require_version('AppIndicator3', '0.1')
            except (ImportError, ValueError):
                os.environ['PYSTRAY_BACKEND'] = 'xorg'

import pystray
from PIL import Image, ImageDraw, ImageFont

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
WINDOW_HOURS = 5
REFRESH_INTERVAL = 30
DEFAULT_LIMIT = 1_000_000
CACHE_READ_WEIGHT = 1 / 150
SETTINGS_FILE = Path.home() / '.claude' / 'menubar_settings.json'
ICON_PATH = Path(__file__).parent / 'claude_icon.png'
SYMBOL_PATH = Path(__file__).parent / 'claude_symbol.png'

# ---------------------------------------------------------------------------
# Settings
# ---------------------------------------------------------------------------

def load_settings():
    try:
        with open(SETTINGS_FILE) as f:
            return json.load(f)
    except Exception:
        return {}


def save_settings(data):
    try:
        with open(SETTINGS_FILE, 'w') as f:
            json.dump(data, f, indent=2)
    except Exception:
        pass


# ---------------------------------------------------------------------------
# Token counting (unchanged core logic)
# ---------------------------------------------------------------------------

def get_usage(window_hours=WINDOW_HOURS):
    base = Path.home() / '.claude' / 'projects'
    now = datetime.now(timezone.utc)
    window_start = now - timedelta(hours=window_hours)

    total_input = total_output = total_cache_create = total_cache_read = 0
    message_count = 0
    seen_request_ids = set()

    for jsonl_path in base.glob('**/*.jsonl'):
        try:
            with open(jsonl_path) as f:
                for line in f:
                    try:
                        d = json.loads(line)
                        if d.get('type') == 'assistant' and isinstance(d.get('message'), dict):
                            ts_str = d.get('timestamp', '')
                            if ts_str:
                                msg_time = datetime.fromisoformat(ts_str.replace('Z', '+00:00'))
                                if msg_time >= window_start:
                                    req_id = d.get('requestId', '')
                                    if req_id and req_id in seen_request_ids:
                                        continue
                                    if req_id:
                                        seen_request_ids.add(req_id)
                                    u = d['message'].get('usage', {})
                                    total_input += u.get('input_tokens', 0)
                                    total_output += u.get('output_tokens', 0)
                                    total_cache_create += u.get('cache_creation_input_tokens', 0)
                                    total_cache_read += u.get('cache_read_input_tokens', 0)
                                    message_count += 1
                    except Exception:
                        pass
        except Exception:
            pass

    return {
        'input': total_input,
        'output': total_output,
        'cache_create': total_cache_create,
        'cache_read': total_cache_read,
        'total': total_input + total_output + total_cache_create + int(total_cache_read * CACHE_READ_WEIGHT),
        'messages': message_count,
    }


def fmt(n):
    if n >= 1_000_000:
        return f"{n / 1_000_000:.2f}M"
    elif n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)


# ---------------------------------------------------------------------------
# Icon generation — monochrome symbol for macOS template rendering
# ---------------------------------------------------------------------------

def _build_symbol():
    """Extract the white Claude symbol from the colored icon PNG.

    Returns a black-on-transparent image. macOS template mode will
    automatically render it black (light mode) or white (dark mode).
    """
    size = 44  # 22pt @2x retina

    # Prefer pre-extracted symbol; fall back to extracting from source icon
    if SYMBOL_PATH.exists():
        src = Image.open(SYMBOL_PATH).convert('RGBA')
    elif ICON_PATH.exists():
        src = Image.open(ICON_PATH).convert('RGBA')
        pixels = src.load()
        out = Image.new('RGBA', src.size, (0, 0, 0, 0))
        op = out.load()
        for y in range(src.height):
            for x in range(src.width):
                r, g, b, _ = pixels[x, y]
                if r + g + b > 600:  # white/light symbol pixels
                    op[x, y] = (0, 0, 0, 255)
        src = out
    else:
        # Fallback: black circle
        img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
        ImageDraw.Draw(img).ellipse([2, 2, size - 2, size - 2], fill=(0, 0, 0, 255))
        return img

    return src.resize((size, size), Image.LANCZOS)


# Build once at import time so make_icon() is fast
_SYMBOL_IMAGE = _build_symbol()


def _white_version(img):
    """Recolor a black-on-transparent symbol to white-on-transparent.

    Linux tray panels are typically dark and have no template-image
    auto-inversion (that's a macOS-only feature), so we render white.
    """
    img = img.convert('RGBA')
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, a = px[x, y]
            px[x, y] = (255, 255, 255, a)
    return img


_WHITE_SYMBOL = _white_version(_SYMBOL_IMAGE)


def _load_font(size):
    for path in (
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
        "/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
    ):
        try:
            return ImageFont.truetype(path, size)
        except Exception:
            continue
    return ImageFont.load_default()


def _render_linux_icon(pct):
    """Draw the Claude symbol + percentage as one image for the Linux tray.

    pystray on Linux can only show an image (no text-next-to-icon API like
    macOS), so the percentage is rendered directly into the icon.
    """
    text = f"{int(round(pct))}%"
    height = 44
    sym_w = 38
    pad = 6
    font = _load_font(32)

    # Measure the text
    tmp = ImageDraw.Draw(Image.new('RGBA', (10, 10)))
    bbox = tmp.textbbox((0, 0), text, font=font)
    tw, th = bbox[2] - bbox[0], bbox[3] - bbox[1]

    width = sym_w + pad + tw + 4
    img = Image.new('RGBA', (width, height), (0, 0, 0, 0))

    sym = _WHITE_SYMBOL.resize((sym_w, sym_w), Image.LANCZOS)
    img.alpha_composite(sym, (0, (height - sym_w) // 2))

    draw = ImageDraw.Draw(img)
    ty = (height - th) // 2 - bbox[1]
    draw.text((sym_w + pad, ty), text, font=font, fill=(255, 255, 255, 255))
    return img


def make_icon(pct=0.0):
    """Return the tray icon image.

    macOS: just the symbol — the percentage is shown as text next to it via
    AppKit (_apply_macos_tweaks). Linux: symbol + percentage rendered together.
    """
    if platform.system() == 'Darwin':
        return _SYMBOL_IMAGE.copy()
    return _render_linux_icon(pct)


def _apply_macos_tweaks(icon, pct):
    """Use AppKit directly to:
      - Enable template mode (auto dark/light adaptation)
      - Show percentage text NEXT TO the icon in the menu bar
    """
    if platform.system() != 'Darwin':
        return
    try:
        import AppKit
        btn = icon._status_item.button()
        # Enable template rendering on the current NSImage
        ns_img = btn.image()
        if ns_img:
            ns_img.setTemplate_(True)
        # Show percentage text to the right of the icon
        btn.setTitle_(f" {int(pct)}%")
        # Ensure image and title are shown side by side
        btn.setImagePosition_(AppKit.NSImageLeft)
    except Exception:
        pass


# ---------------------------------------------------------------------------
# Cross-platform notifications
# ---------------------------------------------------------------------------

def notify(title, subtitle, message):
    try:
        if platform.system() == 'Darwin':
            icon_str = str(ICON_PATH) if ICON_PATH.exists() else ''
            # Try terminal-notifier first (supports custom app icon)
            tn = subprocess.run(['which', 'terminal-notifier'], capture_output=True, text=True)
            if tn.returncode == 0:
                cmd = [
                    'terminal-notifier',
                    '-title', title,
                    '-subtitle', subtitle,
                    '-message', message,
                ]
                if icon_str:
                    cmd += ['-contentImage', icon_str, '-appIcon', icon_str]
                subprocess.run(cmd, check=False)
            else:
                escaped_msg = message.replace('"', '\\"')
                escaped_sub = subtitle.replace('"', '\\"')
                escaped_title = title.replace('"', '\\"')
                script = (
                    f'display notification "{escaped_msg}" '
                    f'with title "{escaped_title}" subtitle "{escaped_sub}"'
                )
                subprocess.run(['osascript', '-e', script], check=False)
        else:
            icon_str = str(ICON_PATH) if ICON_PATH.exists() else ''
            cmd = ['notify-send', '-t', '5000']
            if icon_str:
                cmd += ['-i', icon_str]
            cmd += [title, f"{subtitle}\n{message}"]
            subprocess.run(cmd, check=False)
    except Exception:
        pass


# ---------------------------------------------------------------------------
# Dialogs — osascript on macOS, zenity on Linux
# ---------------------------------------------------------------------------

def ask_input(title, prompt, default=''):
    if platform.system() == 'Darwin':
        escaped_prompt = prompt.replace('"', '\\"').replace('\n', '\\n')
        escaped_default = str(default).replace('"', '\\"')
        script = (
            f'display dialog "{escaped_prompt}" '
            f'with title "{title}" '
            f'default answer "{escaped_default}" '
            f'buttons {{"Cancel", "OK"}} default button "OK"'
        )
        try:
            out = subprocess.check_output(['osascript', '-e', script], text=True).strip()
            for part in out.split(', '):
                if part.startswith('text returned:'):
                    return part[len('text returned:'):]
        except subprocess.CalledProcessError:
            return None
        return None
    else:
        try:
            out = subprocess.check_output([
                'zenity', '--entry',
                '--title', title,
                '--text', prompt,
                '--entry-text', str(default),
            ], text=True).strip()
            return out
        except (subprocess.CalledProcessError, FileNotFoundError):
            return None


def show_alert(title, message):
    if platform.system() == 'Darwin':
        escaped = message.replace('"', '\\"').replace('\n', '\\n')
        script = (
            f'display dialog "{escaped}" '
            f'with title "{title}" '
            f'buttons {{"OK"}} default button "OK"'
        )
        try:
            subprocess.run(['osascript', '-e', script], check=False)
        except Exception:
            pass
    else:
        try:
            subprocess.run([
                'zenity', '--info',
                '--title', title,
                '--text', message,
            ], check=False)
        except FileNotFoundError:
            pass


# ---------------------------------------------------------------------------
# Application state
# ---------------------------------------------------------------------------

_state = {
    'pct': 0.0,
    'total': 0,
    'limit': DEFAULT_LIMIT,
    'messages': 0,
    'input': 0,
    'output': 0,
    'cache_create': 0,
    'cache_read': 0,
}
_alerted = set()
_icon = None
_timer = None


def _load_limit():
    settings = load_settings()
    _state['limit'] = settings.get('limit', DEFAULT_LIMIT)


# ---------------------------------------------------------------------------
# Refresh logic
# ---------------------------------------------------------------------------

def do_refresh():
    global _timer
    usage = get_usage()
    total = usage['total']
    limit = _state['limit']
    pct = min(100.0, (total / limit) * 100.0) if limit > 0 else 0.0

    _state.update({
        'pct': pct,
        'total': total,
        'messages': usage['messages'],
        'input': usage['input'],
        'output': usage['output'],
        'cache_create': usage['cache_create'],
        'cache_read': usage['cache_read'],
    })

    if _icon is not None:
        _icon.icon = make_icon(pct)
        _icon.title = f"Claude Usage: {pct:.0f}%"
        _icon.update_menu()
        _apply_macos_tweaks(_icon, pct)

    # Fire 10% milestone notifications
    crossed = int(pct / 10) * 10
    for milestone in range(10, crossed + 1, 10):
        if milestone not in _alerted and pct >= milestone:
            _alerted.add(milestone)
            if milestone >= 90:
                msg = "Approaching limit — consider pausing!"
            elif milestone >= 70:
                msg = "Getting close to your limit."
            else:
                msg = f"Used {fmt(total)} tokens in the last 5 hours."
            notify("Claude Usage", f"{milestone}% of limit reached", msg)

    # Schedule next refresh
    _timer = threading.Timer(REFRESH_INTERVAL, do_refresh)
    _timer.daemon = True
    _timer.start()


# ---------------------------------------------------------------------------
# Menu callbacks
# ---------------------------------------------------------------------------

def on_set_limit(icon, item):
    val = ask_input("Set Usage Limit", "Enter token limit for 5-hour window\n(e.g. 22800000 for 22.8M):", _state['limit'])
    if val is None:
        return
    try:
        new_limit = int(val.strip().replace(',', '').replace('_', ''))
        if new_limit > 0:
            _state['limit'] = new_limit
            _alerted.clear()
            save_settings({'limit': new_limit})
            do_refresh()
        else:
            show_alert("Invalid value", "Please enter a positive number.")
    except ValueError:
        show_alert("Invalid value", "Please enter a whole number (e.g. 22800000).")


def on_calibrate(icon, item):
    total = _state['total']
    val = ask_input(
        "Calibrate from Website",
        f"Open claude.ai and check your current usage %.\n"
        f"Enter that percentage to calibrate the token limit.\n\n"
        f"Current measured tokens: {fmt(total)}",
        ""
    )
    if val is None or total == 0:
        return
    try:
        site_pct = float(val.strip().rstrip('%'))
        if 0 < site_pct <= 100:
            new_limit = int(total / (site_pct / 100.0))
            _state['limit'] = new_limit
            _alerted.clear()
            save_settings({'limit': new_limit})
            do_refresh()
            show_alert("Calibrated", f"Token limit set to {fmt(new_limit)}\nbased on {site_pct:.1f}% from website.")
        else:
            show_alert("Invalid value", "Enter a percentage between 1 and 100.")
    except ValueError:
        show_alert("Invalid value", "Enter a number like 12 or 12.5.")


def on_reset_alerts(icon, item):
    _alerted.clear()
    notify("Claude Usage", "Alerts reset", "You'll be notified again at each 10% milestone.")


def on_refresh(icon, item):
    threading.Thread(target=do_refresh, daemon=True).start()


def on_quit(icon, item):
    if _timer is not None:
        _timer.cancel()
    icon.stop()


# ---------------------------------------------------------------------------
# Menu item title callables (called on each menu render)
# ---------------------------------------------------------------------------

def t_usage(item):
    return f"Usage: {fmt(_state['total'])} / {fmt(_state['limit'])}  ({_state['pct']:.1f}%)"

def t_messages(item):
    return f"Messages (5h): {_state['messages']}"

def t_input(item):
    return f"  Input:          {fmt(_state['input'])}"

def t_output(item):
    return f"  Output:         {fmt(_state['output'])}"

def t_cache_create(item):
    return f"  Cache created:  {fmt(_state['cache_create'])}"

def t_cache_read(item):
    return f"  Cache read:     {fmt(_state['cache_read'])}"

def t_window(item):
    return f"Window: last {WINDOW_HOURS} hours"

def t_limit(item):
    return f"Limit: {fmt(_state['limit'])} tokens"


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def setup(icon):
    icon.visible = True
    do_refresh()
    # Template mode must be applied after the first icon image is set
    _apply_macos_tweaks(icon, _state['pct'])


def main():
    global _icon
    _load_limit()

    menu = pystray.Menu(
        pystray.MenuItem(t_usage, None, enabled=False),
        pystray.MenuItem(t_messages, None, enabled=False),
        pystray.Menu.SEPARATOR,
        pystray.MenuItem(t_input, None, enabled=False),
        pystray.MenuItem(t_output, None, enabled=False),
        pystray.MenuItem(t_cache_create, None, enabled=False),
        pystray.MenuItem(t_cache_read, None, enabled=False),
        pystray.Menu.SEPARATOR,
        pystray.MenuItem(t_window, None, enabled=False),
        pystray.MenuItem(t_limit, None, enabled=False),
        pystray.MenuItem("Set Limit...", on_set_limit),
        pystray.MenuItem("Calibrate from Website...", on_calibrate),
        pystray.MenuItem("Reset Alerts", on_reset_alerts),
        pystray.Menu.SEPARATOR,
        pystray.MenuItem("Refresh Now", on_refresh),
        pystray.MenuItem("Quit", on_quit),
    )

    _icon = pystray.Icon(
        name="claude_usage",
        icon=make_icon(),
        title="Claude Usage",
        menu=menu,
    )

    _icon.run(setup=setup)


if __name__ == '__main__':
    main()
