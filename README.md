# StackGet 🛠️

**StackGet** is a lightning-fast CLI tool that scans your machine and generates a universal, structured manifest of your entire development stack. 

Modern development requires a fragile house of cards: specific versions of Node, Python, Docker, PostgreSQL, and system utilities. When a teammate's build fails, the culprit is usually a hidden version mismatch. 

Instead of spending an hour on Zoom typing `node -v` and `docker --version`, **StackGet** instantly snapshots your local environment into a clean `yaml` file, allowing you to validate your setup against the project's requirements or diff it against a teammate's machine.

### Why use StackGet?
* 📸 **Snapshot:** Generate a complete environment report with a single `stackget scan`.
* 🤝 **Onboard:** Drop a `stackget.yaml` in your repo so new hires know exactly what tools to install.
* 🕵️ **Debug:** Run `stackget diff env1.yaml env2.yaml` to instantly spot the difference between a working machine and a broken one.
* ✅ **Validate:** Use `stackget check` in your CI/CD pipeline or local git hooks to ensure compatible tooling.
