# RNNoise integration

User approved an embedded open-source model on 2026-10-29 after repeated reports
that the heuristic keyboard filter was insufficient. This is RNNoise by Xiph,
not a neural model trained or invented by Senja.

- Upstream: https://github.com/xiph/rnnoise
- Version: v0.2, commit `904a876dce1f9ab8860c0a5000ed151f9f6eef58`
- Model: https://media.xiph.org/rnnoise/models/rnnoise_data-0b50c45.tar.gz
- Generated model C SHA256: `522b6a64fded05bf85e58c06206eafe57ce7d94f3af58c725b17628b481d7890`
- License: RNNoise-LICENSE.txt, with original source notices preserved.

Build with scripts/build-neural-audio.ps1 using a verified official Zig compiler.
Baseline and SSE4.1/AVX2 neural kernels are compiled separately; upstream CPU
dispatch selects the supported path at runtime. The model is built into
SenjaNoise.dll. No model download or external application runs at runtime.

The shim consumes mono 48kHz signed PCM16, streams arbitrary chunk lengths,
returns aligned speech probability as an integer, and retains no caller buffer.
The 480-sample bridge adds one 10ms frame of queueing in addition to RNNoise's
own algorithmic delay. It never records or uploads audio. It does not register
a virtual microphone or identify the nearest speaker.
