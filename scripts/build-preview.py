"""Build a private-data-free browser preview from the shipped menu source."""
from pathlib import Path
import html
import json

root = Path(__file__).resolve().parent.parent
ui = root / "cmd/samar/ui"
template = (ui / "menu.html").read_text(encoding="utf-8")
css = (ui / "menu.css").read_text(encoding="utf-8")
js = (ui / "commands.js").read_text(encoding="utf-8") + "\n" + (ui / "menu.js").read_text(encoding="utf-8")
frames = []
for label, background in [
    ("Night", "#11151d"),
    ("Color", "linear-gradient(135deg,#3b558c,#894b75 60%,#b76a43)"),
    ("Light", "linear-gradient(135deg,#cbdfe8,#e6cddc)"),
]:
    overlay = "body{background:" + background + "}.panel{opacity:1;backdrop-filter:blur(4px)}"
    bridge = """let previewEngineState={...window.__SENJA_STATE__};
window.chrome={webview:{postMessage(raw){const m=JSON.parse(raw);if(m.type==='command'){
if(m.command>=100&&m.command<=103)previewEngineState.style=m.command-99;
if(m.command===105)previewEngineState.rounded=!previewEngineState.rounded;
if(m.command===110)previewEngineState.startup=!previewEngineState.startup;
setTimeout(()=>window.senjaUpdate({...previewEngineState}),180);
}}}};"""
    doc = template.replace("__CSS__", css + overlay).replace("__STATE__", json.dumps({"style": 1, "rounded": True, "startup": True})).replace("__JS__", bridge + js)
    frames.append('<section><h2>' + label + '</h2><iframe title="Senja ' + label + '" srcdoc="' + html.escape(doc, quote=True) + '"></iframe></section>')
page = '''<!doctype html><html><meta charset="utf-8"><title>Senja Bar — GUI preview</title>
<style>*{box-sizing:border-box}body{margin:0;background:#0e1015;color:#eef1f8;font-family:Segoe UI,sans-serif}header{margin:28px 32px 20px}h1{font-size:25px;margin:0}p{color:#b4becf;margin:6px 0 0;font-size:13px}.gallery{display:flex;gap:24px;margin:0 32px}h2{font-size:12px;font-weight:500;color:#aeb9cc;margin:0 0 10px}iframe{display:block;width:360px;height:548px;border:0;border-radius:28px;box-shadow:0 14px 32px #0004}</style>
<header><h1>Senja Bar</h1><p>One interface. Every background.</p></header><main class="gallery">'''+"".join(frames)+"</main></html>"
output = root / "docs/preview/index.html"
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text(page, encoding="utf-8", newline="\n")
