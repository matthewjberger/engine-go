<h1 align="center">indigo</h1>

<p align="center">
  <a href="https://github.com/matthewjberger/indigo"><img alt="github" src="https://img.shields.io/badge/github-matthewjberger/indigo-8da0cb?style=for-the-badge&labelColor=555555&logo=github" height="20"></a>
  <a href="https://github.com/matthewjberger/indigo/blob/main/LICENSE-MIT"><img alt="license" src="https://img.shields.io/badge/license-MIT%2FApache--2.0-blue?style=for-the-badge&labelColor=555555" height="20"></a>
</p>

<p align="center"><strong>A data-oriented game engine written in Go with a custom ECS. Works on Windows, Linux, macOS, and the web.</strong></p>

Archetype ECS, a wgpu-backed render graph with clustered lighting and image-based PBR, and a retained ECS-driven UI. Runs on GLFW on the desktop and on WebAssembly + canvas in the browser. Engine state lives as components on world entities, systems are plain `func(*ecs.World)` functions, and the renderer reads from the same world the simulation writes to.

The editor ships in this repository as the end-to-end consumer.

[![indigo editor](docs/editor.png)](https://matthewberger.dev/indigo/editor/)

[Editor in the browser](https://matthewberger.dev/indigo/editor/) (WebGPU required).

Architecture notes are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

Dual-licensed under [MIT](LICENSE-MIT) or [Apache-2.0](LICENSE-APACHE) at your option.
