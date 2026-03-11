package detector

import "time"

// ToolDef describes how to detect a single tool.
//
//   Commands      – ordered list of binary names to try (first found wins)
//   VersionArgs   – ordered arg strings to attempt (space-separated); nil → defaultVersionArgs
//   VersionFilter – output must contain this string for the match to count
//                   (used to distinguish python2 vs python3, etc.)
//   VersionRegex  – when non-empty, replaces generic semver scan; must contain exactly
//                   one capturing group that yields the version string.
//                   Use when the default semver regexes pick up the wrong number
//                   (e.g. nmap output embeds a Darwin SDK version before the real one).
//   NoVersion     – if true, just detect presence (no version extraction)
//   Timeout       – per-tool subprocess timeout; 0 = use default (2s)
//   Stdin         – if non-empty, written to the subprocess's stdin (for REPLs like tclsh)
//   MultiVersion  – enumerate all installed versions via version managers / OS dirs
//   GUIApp        – name-based OS app discovery for GUI tools not on $PATH
type ToolDef struct {
	Name          string
	Commands      []string
	VersionArgs   []string
	VersionFilter string
	VersionRegex  string
	NoVersion     bool
	Timeout       time.Duration
	Stdin         string
	MultiVersion  bool
	GUIApp        *GUIApp
}

// GUIApp enables scalable OS app discovery for GUI tools that install to standard
// application directories but are not on $PATH. The scanner searches each platform's
// standard roots using shallow glob patterns (max depth 3), which is as fast as a
// directory listing — no recursive walks.
//
//   Windows roots (evaluated at runtime, never hardcoded):
//     %ProgramFiles%, %ProgramFiles(x86)%, %LOCALAPPDATA%\Programs, + WinHints
//
//   macOS roots:   /Applications, ~/Applications
//   Linux roots:   PATH, /opt/*[/bin], /usr/share/*[/bin], + LinuxHints
type GUIApp struct {
	// WinExe is the .exe (or .bat) filename to search for (e.g. "MongoDBCompass.exe").
	// Searched at depth 2 (root\App\file.exe) and depth 3 (root\Vendor\App\file.exe).
	WinExe string

	// WinHints are extra Windows roots also searched at depth 2 and 3.
	// Use for vendor sub-directories: e.g. `C:\Program Files\JetBrains`
	// locates DataGrip at depth 3 (JetBrains\DataGrip 2024.3\bin\datagrip64.exe).
	WinHints []string

	// MacApp is the .app bundle name (e.g. "MongoDB Compass.app").
	// Version is extracted from Contents/Info.plist via PlistBuddy — zero cost.
	MacApp string

	// LinuxBin is the binary name on Linux (e.g. "mongodb-compass").
	// Searched via PATH, /opt/*/[bin/], /usr/share/*/[bin/], then LinuxHints.
	LinuxBin string

	// LinuxHints are extra Linux directories (glob patterns allowed).
	// e.g. "/usr/pgadmin*/bin" covers distro-versioned pgAdmin paths.
	LinuxHints []string

	// EnvVar is the name of an environment variable whose value is the tool's
	// install root. StackGet probes $EnvVar/bin/<binary> before the file walker.
	// Used for tools like Apache Tomcat (CATALINA_HOME) and Hadoop (HADOOP_HOME)
	// that install to a custom directory pointed to by a well-known env var.
	EnvVar string
}

// Category groups related tools.
type Category struct {
	Name  string
	Emoji string
	Tools []ToolDef
}

// AllCategories is the master list every category and tool StackGet detects.
var AllCategories = []Category{

	// ──────────────────────────────────────────────────────────────
	// LANGUAGES — interpreters, runtimes, and primary toolchains
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Languages",
		Emoji: "🔤",
		Tools: []ToolDef{
			// --- Web / Scripting ---
			{Name: "Node.js", Commands: []string{"node"}, MultiVersion: true},
			// Python 2 and 3 use VersionFilter so they're not confused with each other.
			{Name: "Python 2", Commands: []string{"python2", "python"}, VersionFilter: "Python 2"},
			{Name: "Python 3", Commands: []string{"python3", "python"}, VersionFilter: "Python 3", MultiVersion: true},
			{Name: "Ruby", Commands: []string{"ruby"}},
			{Name: "PHP", Commands: []string{"php"}},
			{Name: "Perl", Commands: []string{"perl"}},
			{Name: "Lua", Commands: []string{"lua", "lua5.4", "lua5.3", "lua5.2"}},

			// --- Systems ---
			{Name: "Go", Commands: []string{"go"}, VersionArgs: []string{"version"}, MultiVersion: true},
			{Name: "Rust (rustc)", Commands: []string{"rustc"}},
			{Name: "Zig", Commands: []string{"zig"}, VersionArgs: []string{"version"}},
			{Name: "Nim", Commands: []string{"nim"}},
			{Name: "Crystal", Commands: []string{"crystal"}},
			{Name: "D (dmd)", Commands: []string{"dmd"}},
			{Name: "D (ldc2)", Commands: []string{"ldc2"}},
			{Name: "V (vlang)", Commands: []string{"v"}},

			// --- JVM ---
			{Name: "Java", Commands: []string{"java"}, VersionArgs: []string{"-version", "--version"}, MultiVersion: true},
			{Name: "Kotlin", Commands: []string{"kotlin"}, VersionArgs: []string{"-version", "--version"}},
			{Name: "Scala", Commands: []string{"scala"}, VersionArgs: []string{"-version", "--version"}},
			{Name: "Groovy", Commands: []string{"groovy"}, VersionArgs: []string{"--version", "-version"}},
			{Name: "Clojure", Commands: []string{"clojure"}},

			// --- .NET ---
			{Name: ".NET", Commands: []string{"dotnet"}},
			{Name: "Mono", Commands: []string{"mono"}},

			// --- Mobile / Cross-platform ---
			{Name: "Dart", Commands: []string{"dart"}, Timeout: 20 * time.Second},
			{Name: "Flutter", Commands: []string{"flutter"}, VersionArgs: []string{"--version"}, VersionFilter: "Flutter", Timeout: 15 * time.Second},
			// swift --version outputs "swift-driver version: 1.120.5 Apple Swift version 6.0.3 ...";
			// semver3 grabs "1.120.5" (swift-driver) before the real Swift version.
			// VersionRegex anchors on "Swift version" to capture the language version.
			{Name: "Swift", Commands: []string{"swift"}, VersionRegex: `Swift version (\d+\.\d+(?:\.\d+)?)`},

			// --- Functional ---
			{Name: "Haskell (GHC)", Commands: []string{"ghc"}},
			// elixir --version embeds the Erlang ERTS version ("15.0.1") before the
			// Elixir version ("1.17.2"); semver3 grabs the ERTS number first.
			// VersionRegex anchors on "Elixir " to capture only the correct number.
			{Name: "Elixir", Commands: []string{"elixir"}, VersionRegex: `Elixir (\d+\.\d+\.\d+)`},
			// erl -version shows ERTS emulator version (e.g. "15.0.1"), not the OTP
		// release number that developers refer to (e.g. "27"). The eval expression
		// queries OTP directly; output is "27" (with Erlang quotes) so VersionRegex
		// strips the quotes. If eval fails, binary presence → "installed".
		{Name: "Erlang", Commands: []string{"erl"},
			VersionArgs:  []string{"-eval erlang:display(erlang:system_info(otp_release)),halt(). -noshell"},
			VersionRegex: `"(\d+)"`,
		},
			{Name: "OCaml", Commands: []string{"ocaml"}},
			{Name: "F# (dotnet fsi)", Commands: []string{"dotnet"}, VersionArgs: []string{"fsi --version"}},
			{Name: "Racket", Commands: []string{"racket"}},
			{Name: "Elm", Commands: []string{"elm"}},
			{Name: "PureScript (spago)", Commands: []string{"spago", "purs"}},
			{Name: "Idris", Commands: []string{"idris", "idris2"}},

			// --- Typed JS/TS ---
			{Name: "TypeScript (tsc)", Commands: []string{"tsc"}},
			{Name: "CoffeeScript", Commands: []string{"coffee"}},

			// --- Scripting / Shell ---
			{Name: "Bash", Commands: []string{"bash"}, VersionArgs: []string{"--version"}, VersionFilter: "GNU bash"},
			// zsh --version on macOS arm64 outputs "zsh 5.9 (arm64-apple-darwin26.3.0)";
			// semver3 grabs "26.3.0" from the Darwin target string before "5.9".
			// VersionRegex anchors on "zsh " to capture only the Zsh version.
			{Name: "Zsh", Commands: []string{"zsh"}, VersionRegex: `zsh (\d+\.\d+(?:\.\d+)?)`},
			{Name: "Fish", Commands: []string{"fish"}},
			{Name: "Nushell", Commands: []string{"nu"}},
			{Name: "PowerShell", Commands: []string{"pwsh", "powershell"}, Timeout: 15 * time.Second},
			{Name: "Tcl", Commands: []string{"tclsh", "tclsh8.6"}, Stdin: "puts [info patchlevel]; exit"},
			{Name: "Gawk", Commands: []string{"gawk"}},
			{Name: "Sed", Commands: []string{"sed"}},

			// --- C / C++ / Systems compilers (primary language toolchain entry) ---
			{Name: "GCC", Commands: []string{"gcc"}},
			{Name: "Clang", Commands: []string{"clang"}},

			// --- Legacy / Academic ---
			{Name: "Julia", Commands: []string{"julia"}},
			{Name: "R", Commands: []string{"Rscript", "R"}},
			{Name: "Octave", Commands: []string{"octave"}},
			{Name: "Wolfram Script", Commands: []string{"wolframscript"}},
			{Name: "COBOL (cobc)", Commands: []string{"cobc"}},
			{Name: "Ada (gnat)", Commands: []string{"gnat"}},
			{Name: "Pascal (fpc)", Commands: []string{"fpc"}},
			{Name: "GFortran", Commands: []string{"gfortran"}},
			{Name: "SWI-Prolog", Commands: []string{"swipl"}},
			{Name: "SBCL (Lisp)", Commands: []string{"sbcl"}},
			{Name: "Guile (Scheme)", Commands: []string{"guile"}},
			{Name: "MIT Scheme", Commands: []string{"mit-scheme", "scheme"}},
			{Name: "SML", Commands: []string{"sml", "mosml"}},
			{Name: "GNU Smalltalk", Commands: []string{"gst"}},

			// --- Emerging / Niche ---
			{Name: "Gleam", Commands: []string{"gleam"}},
			{Name: "Grain", Commands: []string{"grain"}},
			{Name: "Roc", Commands: []string{"roc"}},
			{Name: "Ballerina", Commands: []string{"bal", "ballerina"}},
			{Name: "Solidity (solc)", Commands: []string{"solc"}},
			{Name: "Vyper", Commands: []string{"vyper"}},
			{Name: "wasm-pack", Commands: []string{"wasm-pack"}},
			{Name: "wat2wasm", Commands: []string{"wat2wasm"}},
			{Name: "NVCC (CUDA)", Commands: []string{"nvcc"}},
			{Name: "Chapel", Commands: []string{"chpl"}},
			{Name: "Iverilog (Verilog)", Commands: []string{"iverilog"}},
			{Name: "GHDL (VHDL)", Commands: []string{"ghdl"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// COMPILERS & BUILD TOOLS  (merged category)
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Compilers & Build Tools",
		Emoji: "🔨",
		Tools: []ToolDef{
			// --- Low-level / Cross-compilation (not duplicated in Languages) ---
			{Name: "Clang++", Commands: []string{"clang++"}},
			{Name: "LLC (LLVM)", Commands: []string{"llc"}},
			{Name: "llvm-as", Commands: []string{"llvm-as"}},
			{Name: "MSVC (cl.exe)", Commands: []string{"cl"}},
			{Name: "NASM", Commands: []string{"nasm"}},
			{Name: "YASM", Commands: []string{"yasm"}},
			{Name: "FASM", Commands: []string{"fasm"}},
			{Name: "GNU as", Commands: []string{"as"}},
			{Name: "Emscripten (emcc)", Commands: []string{"emcc"}},
			// swiftc --version has the same darwin26 contamination as swift --version:
		// "swift-driver version: 1.120.5 Apple Swift version 6.0.3 ... darwin26.3.0"
		// semver3 grabs "1.120.5" (swift-driver) before the real Swift version.
		{Name: "Swiftc", Commands: []string{"swiftc"}, VersionRegex: `Swift version (\d+\.\d+(?:\.\d+)?)`},
			{Name: "Rustc", Commands: []string{"rustc"}},
			{Name: "Javac", Commands: []string{"javac"}, VersionArgs: []string{"-version", "--version"}},
			{Name: "Kotlinc", Commands: []string{"kotlinc"}},
			{Name: "Scalac", Commands: []string{"scalac"}},
			{Name: "MCS (Mono C#)", Commands: []string{"mcs"}},
			{Name: "arm-linux-gnueabi-gcc", Commands: []string{"arm-linux-gnueabi-gcc"}},
			{Name: "aarch64-linux-gnu-gcc", Commands: []string{"aarch64-linux-gnu-gcc"}},
			{Name: "x86_64-w64-mingw32-gcc", Commands: []string{"x86_64-w64-mingw32-gcc"}},
			{Name: "wasi-cc", Commands: []string{"wasi-cc", "wasm32-wasi-cc"}},

			// --- Generic Build Systems ---
			// make --version on macOS arm64 includes "arm-apple-darwinXX.Y.Z" after the
		// real "GNU Make X.Y" line; semver3 picks up the Darwin SDK string first.
		// VersionRegex anchors on "GNU Make" to capture only the correct number.
		{Name: "Make", Commands: []string{"make"}, VersionRegex: `GNU Make (\d+\.\d+(?:\.\d+)?)`},
			{Name: "CMake", Commands: []string{"cmake"}},
			{Name: "Ninja", Commands: []string{"ninja"}},
			{Name: "Meson", Commands: []string{"meson"}},
			{Name: "Autoconf", Commands: []string{"autoconf"}},
			{Name: "Automake", Commands: []string{"automake"}},
			{Name: "Libtool", Commands: []string{"libtool"}},
			// pkg-config --version outputs a bare semver (e.g. "0.29.2") — it does
			// have a version flag; NoVersion was wrong.
			{Name: "pkg-config", Commands: []string{"pkg-config"}},
			{Name: "Bazel", Commands: []string{"bazel", "bazelisk"}},
			// buck2 --version outputs a date string like "buck2 2024-04-13" which
			// contains dashes, not dots — no semver match is possible without a regex.
			{Name: "Buck2", Commands: []string{"buck2"}, VersionRegex: `buck2 (\S+)`},
			{Name: "Pants", Commands: []string{"pants"}},
			{Name: "Please", Commands: []string{"please"}},
			{Name: "Just", Commands: []string{"just"}},
			{Name: "Task (go-task)", Commands: []string{"task"}},
			{Name: "Tusk", Commands: []string{"tusk"}},
			{Name: "Rake", Commands: []string{"rake"}},
			// MSBuild uses /version (Windows flag style); --version is the Linux/Mono form.
			{Name: "MSBuild", Commands: []string{"msbuild"}, VersionArgs: []string{"/version", "--version", "-version"}},
			// xcodebuild only accepts -version (single dash); --version and bare "version"
		// either error out or start a build action which hangs the timeout.
		// CLT-only installs: -version prints an error with no parseable number, so
		// GUIApp reads the version from /Applications/Xcode.app/Contents/Info.plist.
		{
			Name:        "xcodebuild",
			Commands:    []string{"xcodebuild"},
			VersionArgs: []string{"-version"},
			GUIApp:      &GUIApp{MacApp: "Xcode.app"},
		},

			// --- JS/TS Bundlers ---
			{Name: "Webpack", Commands: []string{"webpack"}},
			{Name: "Vite", Commands: []string{"vite"}},
			{Name: "esbuild", Commands: []string{"esbuild"}},
			{Name: "Parcel", Commands: []string{"parcel"}},
			{Name: "Rollup", Commands: []string{"rollup"}},
			{Name: "Turbo", Commands: []string{"turbo"}},
			{Name: "Nx", Commands: []string{"nx"}},
			{Name: "Gulp", Commands: []string{"gulp"}},
			{Name: "Grunt", Commands: []string{"grunt"}},
			{Name: "Brunch", Commands: []string{"brunch"}},

			// --- JVM Build ---
			{Name: "Maven (mvn)", Commands: []string{"mvn"}},
			{Name: "Gradle", Commands: []string{"gradle"}},
			{Name: "Ant", Commands: []string{"ant"}},
			{Name: "SBT", Commands: []string{"sbt"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// PACKAGE MANAGERS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Package Managers",
		Emoji: "📦",
		Tools: []ToolDef{
			// JS/TS (npx removed — ships with npm)
			{Name: "npm", Commands: []string{"npm"}},
			{Name: "Yarn", Commands: []string{"yarn"}},
			{Name: "pnpm", Commands: []string{"pnpm"}},
			{Name: "Bun", Commands: []string{"bun"}},
			{Name: "Deno", Commands: []string{"deno"}},

			// Python (pip3 removed — pip handles both; pip3 identical version on modern Python)
			{Name: "pip", Commands: []string{"pip", "pip3"}},
			{Name: "pipenv", Commands: []string{"pipenv"}},
			{Name: "Poetry", Commands: []string{"poetry"}},
			{Name: "Conda", Commands: []string{"conda"}},
			{Name: "uv", Commands: []string{"uv"}},
			{Name: "PDM", Commands: []string{"pdm"}},
			{Name: "Hatch", Commands: []string{"hatch"}},
			{Name: "pipx", Commands: []string{"pipx"}},

			// Rust
			{Name: "Cargo", Commands: []string{"cargo"}},

			// Ruby
			{Name: "Gem", Commands: []string{"gem"}},
			{Name: "Bundler", Commands: []string{"bundle", "bundler"}},

			// PHP
			{Name: "Composer", Commands: []string{"composer"}},

			// .NET
			// nuget with no args (or "help") prints "NuGet Version: x.x.x"; --version
			// is not a recognized flag. VersionRegex anchors on the "NuGet Version:" prefix.
			{Name: "NuGet", Commands: []string{"nuget"}, VersionArgs: []string{"help", "--version"}, VersionRegex: `NuGet Version: (\d+\.\d+\.\d+)`},
			{Name: "Paket", Commands: []string{"paket"}},

			// Elixir / Erlang
			{Name: "Mix (Elixir)", Commands: []string{"mix"}},
			{Name: "Rebar3 (Erlang)", Commands: []string{"rebar3"}},

			// Haskell
			{Name: "Cabal", Commands: []string{"cabal"}},
			{Name: "Stack", Commands: []string{"stack"}},
			{Name: "GHCup", Commands: []string{"ghcup"}},

			// Dart / Flutter
			// "dart pub --version" is the canonical command; fall back to "dart --version"
			// which also shows the SDK version (Pub is bundled with Dart SDK).
			{Name: "Pub (Dart)", Commands: []string{"dart"}, VersionArgs: []string{"pub --version", "--version"}, Timeout: 20 * time.Second},

			// OCaml
			{Name: "OPAM", Commands: []string{"opam"}},
			{Name: "Esy", Commands: []string{"esy"}},

			// Nim / Crystal
			{Name: "Nimble", Commands: []string{"nimble"}},
			{Name: "Shards (Crystal)", Commands: []string{"shards"}},

			// Clojure
			{Name: "Leiningen", Commands: []string{"lein"}},
			{Name: "Boot (Clojure)", Commands: []string{"boot"}},

			// C/C++
			{Name: "Conan", Commands: []string{"conan"}},
			{Name: "vcpkg", Commands: []string{"vcpkg"}},
			{Name: "XMake", Commands: []string{"xmake"}},

			// OS Package Managers
			{Name: "Homebrew", Commands: []string{"brew"}},
			{Name: "apt", Commands: []string{"apt"}},
			{Name: "apt-get", Commands: []string{"apt-get"}},
			{Name: "yum", Commands: []string{"yum"}, VersionArgs: []string{"--version", "version"}},
			{Name: "dnf", Commands: []string{"dnf"}},
			{Name: "pacman", Commands: []string{"pacman"}, VersionArgs: []string{"--version", "-V"}},
			{Name: "apk (Alpine)", Commands: []string{"apk"}, VersionArgs: []string{"--version", "version"}},
			{Name: "zypper", Commands: []string{"zypper"}},
			{Name: "emerge (Portage)", Commands: []string{"emerge"}},
			{Name: "Nix", Commands: []string{"nix"}},
			{Name: "Guix", Commands: []string{"guix"}},
			{Name: "Scoop", Commands: []string{"scoop"}},
			{Name: "Chocolatey", Commands: []string{"choco"}},
			{Name: "winget", Commands: []string{"winget"}},
			{Name: "Snap", Commands: []string{"snap"}},
			{Name: "Flatpak", Commands: []string{"flatpak"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// DATABASES
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Databases",
		Emoji: "🗄️",
		Tools: []ToolDef{
			{Name: "PostgreSQL (psql)", Commands: []string{"psql"}},
			{Name: "MySQL", Commands: []string{"mysql"}},
			{Name: "MariaDB", Commands: []string{"mariadb"}},
			{Name: "Redis CLI", Commands: []string{"redis-cli"}},
			{Name: "MongoDB (mongod)", Commands: []string{"mongod"}},
			{Name: "mongosh", Commands: []string{"mongosh"}},
			{Name: "SQLite3", Commands: []string{"sqlite3"}},
			{Name: "Cassandra (cqlsh)", Commands: []string{"cqlsh"}},
			{Name: "CockroachDB", Commands: []string{"cockroach"}},
			{Name: "InfluxDB (influx)", Commands: []string{"influx"}},
			{Name: "ClickHouse", Commands: []string{"clickhouse", "clickhouse-client"}},
			{Name: "DuckDB", Commands: []string{"duckdb"}},
			{Name: "MSSQL (sqlcmd)", Commands: []string{"sqlcmd"}},
			{Name: "Oracle (sqlplus)", Commands: []string{"sqlplus"}},
			{Name: "etcdctl", Commands: []string{"etcdctl"}},
			{Name: "Kafka (kafka-topics)", Commands: []string{"kafka-topics.sh", "kafka-topics"}},
			{Name: "Turso CLI", Commands: []string{"turso"}},
			{Name: "PlanetScale (pscale)", Commands: []string{"pscale"}},
			{Name: "Supabase CLI", Commands: []string{"supabase"}},
			{Name: "Prisma", Commands: []string{"prisma"}},
			{Name: "Drizzle-Kit", Commands: []string{"drizzle-kit"}},
			{Name: "Flyway", Commands: []string{"flyway"}},
			{Name: "Liquibase", Commands: []string{"liquibase"}},
			{Name: "Alembic", Commands: []string{"alembic"}},
			{Name: "SurrealDB", Commands: []string{"surreal"}},
			{Name: "EdgeDB", Commands: []string{"edgedb"}},
			{Name: "Neo4j (cypher-shell)", Commands: []string{"cypher-shell"}},
			{Name: "ArangoDB (arangosh)", Commands: []string{"arangosh"}},
			{Name: "RethinkDB", Commands: []string{"rethinkdb"}},
			{Name: "Prometheus", Commands: []string{"prometheus"}},
		{Name: "Snowflake CLI (snowsql)", Commands: []string{"snowsql"}, VersionArgs: []string{"-v", "--version"}},
		{Name: "RabbitMQ", Commands: []string{"rabbitmqctl"}, VersionArgs: []string{"version"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// GUI DATABASE CLIENTS — scalable discovery via GUIApp (no hardcoded paths)
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "GUI Database Clients",
		Emoji: "🖥️",
		Tools: []ToolDef{
			{
				Name:     "pgAdmin 4",
				Commands: []string{"pgadmin4"},
				// Windows: C:\Program Files\pgAdmin 4\runtime\pgadmin4.exe
				//   WinHint points at the app dir → depth-2 search finds \runtime\pgadmin4.exe.
				// Linux: distro packages place it under /usr/pgadmin<N>/bin/
				GUIApp: &GUIApp{
					WinExe:     "pgadmin4.exe",
					WinHints:   []string{`C:\Program Files\pgAdmin 4`},
					MacApp:     "pgAdmin 4.app",
					LinuxBin:   "pgadmin4",
					LinuxHints: []string{"/usr/pgadmin*/bin"},
				},
			},
			{
				Name:     "MySQL Workbench",
				Commands: []string{"mysql-workbench", "mysqlworkbench"},
				// Windows: C:\Program Files\MySQL\MySQL Workbench 8.0\MySQLWorkbench.exe
				//   WinHints point at MySQL vendor dirs → depth-2 finds \Workbench 8.0\exe.
				//   versionFromPath then extracts "8.0" from the directory name.
				GUIApp: &GUIApp{
					WinExe: "MySQLWorkbench.exe",
					WinHints: []string{
						`C:\Program Files\MySQL`,
						`C:\Program Files (x86)\MySQL`,
					},
					MacApp:   "MySQLWorkbench.app",
					LinuxBin: "mysql-workbench",
				},
			},
			{
				// DBeaver: Linux/macOS binary responds to --version.
				// Windows dbeaver.exe opens the GUI so we skip exec and use versionFromPath.
				Name:     "DBeaver",
				Commands: []string{"dbeaver", "dbeaver-ce"},
				GUIApp: &GUIApp{
					WinExe:   "dbeaver.exe",
					MacApp:   "DBeaver.app",
					LinuxBin: "dbeaver",
					LinuxHints: []string{
						"/usr/share/dbeaver-ce",
						"/opt/dbeaver",
						"/opt/dbeaver-ce",
						"/snap/dbeaver-ce/current/usr/share/dbeaver-ce",
					},
				},
			},
			{
				// DataGrip: C:\Program Files\JetBrains\DataGrip 2024.3\bin\datagrip64.exe
				// WinHint is a GLOB pattern — expandHint() expands it to matching version
				// dirs, then depth-1 from hint\bin\ finds the exe instantly.
				// versionFromPath extracts "2024.3" from the resolved path.
				Name:     "DataGrip",
				Commands: []string{"datagrip"},
				GUIApp: &GUIApp{
					WinExe:   "datagrip64.exe",
					WinHints: []string{`C:\Program Files\JetBrains\DataGrip *\bin`},
					MacApp:   "DataGrip.app",
					LinuxBin: "datagrip.sh",
					LinuxHints: []string{
						"/opt/datagrip*/bin",
						"/opt/DataGrip*/bin",
					},
				},
			},
			{
				// MongoDB Compass (Electron). Installed per-user on Windows:
				//   %LOCALAPPDATA%\Programs\MongoDB\MongoDB Compass\MongoDBCompass.exe  (depth 3)
				// or system-wide:
				//   C:\Program Files\MongoDB Compass\MongoDBCompass.exe               (depth 2)
				// Both are covered by the standard roots + depth 2/3 search.
				Name:     "MongoDB Compass",
				Commands: []string{"mongodb-compass", "MongoDBCompass"},
				GUIApp: &GUIApp{
					WinExe:   "MongoDBCompass.exe",
					MacApp:   "MongoDB Compass.app",
					LinuxBin: "mongodb-compass",
					LinuxHints: []string{
						"/opt/mongodb-compass",
						"/usr/share/mongodb-compass",
					},
				},
			},
			{
				Name:     "TablePlus",
				Commands: []string{"tableplus"},
				GUIApp: &GUIApp{
					WinExe:   "TablePlus.exe",
					MacApp:   "TablePlus.app",
					LinuxBin: "tableplus",
				},
			},
			{
				Name:     "HeidiSQL",
				Commands: []string{"heidisql"},
				GUIApp: &GUIApp{
					WinExe:   "heidisql.exe",
					WinHints: []string{`C:\Program Files\HeidiSQL`},
				},
			},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// DEVOPS & INFRASTRUCTURE
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "DevOps & Infrastructure",
		Emoji: "🚀",
		Tools: []ToolDef{
			{Name: "Docker", Commands: []string{"docker"}},
			{Name: "Docker Compose", Commands: []string{"docker-compose"}, VersionArgs: []string{"--version", "version"}},
			{Name: "Docker Buildx", Commands: []string{"docker"}, VersionArgs: []string{"buildx version"}},
			// Docker Desktop: GUI app; CLI is just "docker" above. Detected separately.
			// Windows: C:\Program Files\Docker\Docker\Docker Desktop.exe
			//   WinHint "C:\Program Files\Docker" → depth-2 finds \Docker\Docker Desktop.exe.
			// macOS:   /Applications/Docker.app  (PlistBuddy reads real version from plist)
			{
				Name:     "Docker Desktop",
				Commands: []string{"docker-desktop"},
				GUIApp: &GUIApp{
					WinExe:   "Docker Desktop.exe",
					WinHints: []string{`C:\Program Files\Docker`},
					MacApp:   "Docker.app",
					LinuxBin: "docker-desktop",
					LinuxHints: []string{
						"/opt/docker-desktop/bin",
					},
				},
			},
			{Name: "Podman", Commands: []string{"podman"}},
			{Name: "Buildah", Commands: []string{"buildah"}},
			{Name: "Skopeo", Commands: []string{"skopeo"}},
			// kubectl --version tries to contact the cluster; "version --client" is
			// the local-only flag that never hangs on network timeouts.
			{Name: "kubectl", Commands: []string{"kubectl"}, VersionArgs: []string{"version --client", "--version"}},
			{Name: "Helm", Commands: []string{"helm"}},
			{Name: "Kustomize", Commands: []string{"kustomize"}},
			{Name: "k9s", Commands: []string{"k9s"}},
			{Name: "Stern", Commands: []string{"stern"}},
			// kubectx and kubens have no version flag — detect by binary presence only.
			{Name: "kubectx", Commands: []string{"kubectx"}, NoVersion: true},
			{Name: "kubens", Commands: []string{"kubens"}, NoVersion: true},
			{Name: "Minikube", Commands: []string{"minikube"}},
			{Name: "Kind", Commands: []string{"kind"}},
			{Name: "k3s", Commands: []string{"k3s"}},
			{Name: "k3d", Commands: []string{"k3d"}},
			{Name: "Terraform", Commands: []string{"terraform"}},
			{Name: "Terragrunt", Commands: []string{"terragrunt"}},
			{Name: "OpenTofu", Commands: []string{"tofu"}},
			{Name: "Ansible", Commands: []string{"ansible"}},
			{Name: "ansible-playbook", Commands: []string{"ansible-playbook"}},
			{Name: "ansible-vault", Commands: []string{"ansible-vault"}},
			{Name: "Puppet", Commands: []string{"puppet"}},
			{Name: "Chef", Commands: []string{"chef"}},
			{Name: "Salt", Commands: []string{"salt", "salt-master"}},
			{Name: "Vagrant", Commands: []string{"vagrant"}},
			{Name: "Packer", Commands: []string{"packer"}},
			{Name: "QEMU", Commands: []string{"qemu-system-x86_64", "qemu-x86_64"}},
			{Name: "AWS CLI", Commands: []string{"aws"}},
			{Name: "Azure CLI (az)", Commands: []string{"az"}},
			{Name: "Google Cloud (gcloud)", Commands: []string{"gcloud"}},
			{Name: "IBM Cloud CLI", Commands: []string{"ibmcloud"}},
			{Name: "OCI CLI", Commands: []string{"oci"}},
			{Name: "Pulumi", Commands: []string{"pulumi"}},
			// "argocd version" contacts the server; "--client" flag is local-only.
			{Name: "ArgoCD CLI", Commands: []string{"argocd"}, VersionArgs: []string{"version --client", "--version"}},
			{Name: "Flux CLI", Commands: []string{"flux"}},
			// "istioctl version" contacts the mesh; "--remote=false" is local-only.
			{Name: "Istioctl", Commands: []string{"istioctl"}, VersionArgs: []string{"version --remote=false", "--version"}},
			// "linkerd version" contacts the cluster; "--client" flag is local-only.
			{Name: "Linkerd CLI", Commands: []string{"linkerd"}, VersionArgs: []string{"version --client", "--version"}},
			{Name: "Consul", Commands: []string{"consul"}},
			{Name: "Vault (HashiCorp)", Commands: []string{"vault"}},
			{Name: "Nomad", Commands: []string{"nomad"}},
			{Name: "Boundary", Commands: []string{"boundary"}},
			{Name: "Waypoint", Commands: []string{"waypoint"}},
			// "oc version" contacts the cluster; "--client" flag is local-only.
			{Name: "OpenShift (oc)", Commands: []string{"oc"}, VersionArgs: []string{"version --client", "--version"}},
		{Name: "WSL", Commands: []string{"wsl"}, VersionArgs: []string{"--version"}},
		{
			// VBoxManage --version outputs "7.1.6r167084" (version + 'r' + build number).
			// On ARM macOS, initialization warnings may precede the version line and
			// contain other 3-part numbers that semver3 would grab first.
			// VersionRegex anchors on the 'r<digits>' suffix so only the VBox version
			// is captured; GUIApp lets the plist supply the version when CLI fails.
			Name:         "VirtualBox",
			Commands:     []string{"VBoxManage", "vboxmanage"},
			VersionArgs:  []string{"--version"},
			VersionRegex: `(\d+\.\d+\.\d+)r\d+`,
			GUIApp: &GUIApp{
				WinExe:   "VBoxManage.exe",
				WinHints: []string{`C:\Program Files\Oracle\VirtualBox`},
				MacApp:   "VirtualBox.app",
				LinuxBin: "VBoxManage",
			},
		},
		{Name: "Supervisor", Commands: []string{"supervisord", "supervisorctl"}, VersionArgs: []string{"-v", "--version"}},
		{Name: "OpenTelemetry Collector", Commands: []string{"otelcol", "otelcol-contrib"}},
		// "grafana --version" works on newer installs; "grafana-cli --version" on older.
	{Name: "Grafana CLI", Commands: []string{"grafana", "grafana-cli"}, VersionArgs: []string{"--version", "-v"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// CI/CD TOOLS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "CI/CD Tools",
		Emoji: "🔄",
		Tools: []ToolDef{
			{Name: "Git", Commands: []string{"git"}},
			{Name: "GitHub CLI (gh)", Commands: []string{"gh"}},
			// GitHub Desktop: GUI app installed per-user on Windows/macOS.
			// Windows: %LOCALAPPDATA%\Programs\GitHub Desktop\GitHubDesktop.exe  (depth 2)
			{
				Name:     "GitHub Desktop",
				Commands: []string{"github-desktop"},
				GUIApp: &GUIApp{
					WinExe:   "GitHubDesktop.exe",
					MacApp:   "GitHub Desktop.app",
					LinuxBin: "github-desktop",
					LinuxHints: []string{
						"/opt/github-desktop",
						"/usr/share/github-desktop",
					},
				},
			},
			{
				Name:     "GitKraken",
				Commands: []string{"gitkraken"},
				GUIApp: &GUIApp{
					WinExe:   "gitkraken.exe",
					MacApp:   "GitKraken.app",
					LinuxBin: "gitkraken",
					LinuxHints: []string{"/opt/gitkraken", "/usr/share/gitkraken"},
				},
			},
			{
				Name:     "Sourcetree",
				Commands: []string{"sourcetree"},
				GUIApp: &GUIApp{
					WinExe: "SourceTree.exe",
					MacApp: "Sourcetree.app",
				},
			},
			{Name: "GitLab CLI (glab)", Commands: []string{"glab"}},
			{Name: "Hub", Commands: []string{"hub"}},
			{Name: "Gitea CLI (tea)", Commands: []string{"tea"}},
			{Name: "GoReleaser", Commands: []string{"goreleaser"}},
			{Name: "Ko", Commands: []string{"ko"}},
			{Name: "Buildpacks (pack)", Commands: []string{"pack"}},
			{Name: "Skaffold", Commands: []string{"skaffold"}},
			{Name: "Tilt", Commands: []string{"tilt"}},
			{Name: "Garden", Commands: []string{"garden"}},
			{Name: "Drone CLI", Commands: []string{"drone"}},
			{Name: "Tekton CLI (tkn)", Commands: []string{"tkn"}},
			{Name: "Act (local GH Actions)", Commands: []string{"act"}},
			{Name: "GitLab Runner", Commands: []string{"gitlab-runner"}},
			{Name: "Buildkite Agent", Commands: []string{"buildkite-agent"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// CLOUD & SERVERLESS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Cloud & Serverless",
		Emoji: "☁️",
		Tools: []ToolDef{
			{Name: "Vercel CLI", Commands: []string{"vercel"}},
			{Name: "Netlify CLI", Commands: []string{"netlify"}},
			{Name: "Railway CLI", Commands: []string{"railway"}},
			{Name: "Fly.io (flyctl)", Commands: []string{"flyctl", "fly"}},
			{Name: "Serverless Framework", Commands: []string{"serverless", "sls"}},
			{Name: "AWS SAM CLI", Commands: []string{"sam"}},
			{Name: "AWS CDK", Commands: []string{"cdk"}},
			{Name: "SST CLI", Commands: []string{"sst"}},
			{Name: "Firebase CLI", Commands: []string{"firebase"}},
			{Name: "Wrangler (Cloudflare)", Commands: []string{"wrangler"}},
			{Name: "Miniflare", Commands: []string{"miniflare"}},
			{Name: "Xata CLI", Commands: []string{"xata"}},
			{Name: "Neon CLI", Commands: []string{"neon"}},
		{Name: "Heroku CLI", Commands: []string{"heroku"}},
		{Name: "DigitalOcean CLI (doctl)", Commands: []string{"doctl"}, VersionArgs: []string{"version"}},
		{Name: "LocalStack", Commands: []string{"localstack"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// RUNTIME MANAGERS & VERSION TOOLS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Runtime Managers & Version Tools",
		Emoji: "🔀",
		Tools: []ToolDef{
			{Name: "nvm", Commands: []string{"nvm"}},
			{Name: "fnm", Commands: []string{"fnm"}},
			{Name: "Volta", Commands: []string{"volta"}},
			{Name: "nodenv", Commands: []string{"nodenv"}},
			{Name: "pyenv", Commands: []string{"pyenv"}},
			{Name: "virtualenv", Commands: []string{"virtualenv"}},
			{Name: "rbenv", Commands: []string{"rbenv"}},
			{Name: "rvm", Commands: []string{"rvm"}},
			{Name: "chruby", Commands: []string{"chruby"}},
			{Name: "SDKMAN (sdk)", Commands: []string{"sdk"}},
			{Name: "jabba", Commands: []string{"jabba"}},
			{Name: "jenv", Commands: []string{"jenv"}},
			{Name: "rustup", Commands: []string{"rustup"}},
			{Name: "gvm", Commands: []string{"gvm"}},
			{Name: "goenv", Commands: []string{"goenv"}},
			{Name: "asdf", Commands: []string{"asdf"}},
			{Name: "mise", Commands: []string{"mise"}},
			{Name: "rtx", Commands: []string{"rtx"}},
			{Name: "proto", Commands: []string{"proto"}},
			{Name: "direnv", Commands: []string{"direnv"}},
		{Name: "PM2", Commands: []string{"pm2"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// SECURITY & CRYPTOGRAPHY
	// (OpenSSL only here — not duplicated in Networking)
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Security & Cryptography",
		Emoji: "🔐",
		Tools: []ToolDef{
			{Name: "OpenSSL", Commands: []string{"openssl"}},
			{Name: "Trivy", Commands: []string{"trivy"}},
			{Name: "Snyk", Commands: []string{"snyk"}},
			{Name: "Grype", Commands: []string{"grype"}},
			{Name: "Syft", Commands: []string{"syft"}},
			{Name: "Cosign", Commands: []string{"cosign"}},
			{Name: "Semgrep", Commands: []string{"semgrep"}},
			{Name: "CodeQL", Commands: []string{"codeql"}},
			{Name: "SOPS", Commands: []string{"sops"}},
			{Name: "age", Commands: []string{"age"}},
			{Name: "GPG", Commands: []string{"gpg", "gpg2"}},
			{Name: "Certbot", Commands: []string{"certbot"}},
			{Name: "step CLI", Commands: []string{"step"}},
			{Name: "cfssl", Commands: []string{"cfssl"}},
			{Name: "mkcert", Commands: []string{"mkcert"}},
			// nmap -V embeds the Darwin SDK version (e.g. "arm-apple-darwin26.x.x") in its
		// output which semver3 matches before the real version ("7.95"). VersionRegex
		// anchors on the "Nmap version" prefix so only the correct number is captured.
		{Name: "nmap", Commands: []string{"nmap"}, VersionArgs: []string{"-V"}, VersionRegex: `Nmap version (\d+\.\d+)`},
			{Name: "masscan", Commands: []string{"masscan"}},
			{Name: "rustscan", Commands: []string{"rustscan"}},
			{Name: "tshark", Commands: []string{"tshark"}},
			{Name: "tcpdump", Commands: []string{"tcpdump"}},
			// Nikto uses --Version (capital V); lowercase --version is not recognized.
			{Name: "Nikto", Commands: []string{"nikto"}, VersionArgs: []string{"--Version", "-Version", "--version"}},
			{Name: "Nuclei", Commands: []string{"nuclei"}},
			{Name: "gitleaks", Commands: []string{"gitleaks"}},
			{Name: "trufflehog", Commands: []string{"trufflehog"}},
			{Name: "Checkov", Commands: []string{"checkov"}},
			{Name: "Terrascan", Commands: []string{"terrascan"}},
			{Name: "tfsec", Commands: []string{"tfsec"}},
			{Name: "Bandit", Commands: []string{"bandit"}},
			{Name: "Safety", Commands: []string{"safety"}},
			{Name: "SonarQube Scanner", Commands: []string{"sonar-scanner"}},
			{Name: "Vault (HashiCorp)", Commands: []string{"vault"}},
		{Name: "1Password CLI (op)", Commands: []string{"op"}},
		{Name: "Bitwarden CLI (bw)", Commands: []string{"bw"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// CODE QUALITY & FORMATTING
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Code Quality & Formatting",
		Emoji: "✨",
		Tools: []ToolDef{
			{Name: "ESLint", Commands: []string{"eslint"}},
			{Name: "Prettier", Commands: []string{"prettier"}},
			{Name: "Stylelint", Commands: []string{"stylelint"}},
			{Name: "Biome", Commands: []string{"biome"}},
			{Name: "Black (Python)", Commands: []string{"black"}},
			{Name: "Ruff", Commands: []string{"ruff"}},
			{Name: "Flake8", Commands: []string{"flake8"}},
			{Name: "Pylint", Commands: []string{"pylint"}},
			{Name: "mypy", Commands: []string{"mypy"}},
			{Name: "pyright", Commands: []string{"pyright"}},
			{Name: "RuboCop", Commands: []string{"rubocop"}},
			{Name: "golangci-lint", Commands: []string{"golangci-lint"}},
			{Name: "staticcheck", Commands: []string{"staticcheck"}},
			{Name: "revive", Commands: []string{"revive"}},
			{Name: "rustfmt", Commands: []string{"rustfmt"}},
			{Name: "Clippy", Commands: []string{"cargo"}, VersionArgs: []string{"clippy --version"}},
			{Name: "clang-format", Commands: []string{"clang-format"}},
			{Name: "clang-tidy", Commands: []string{"clang-tidy"}},
			{Name: "cppcheck", Commands: []string{"cppcheck"}},
			{Name: "Valgrind", Commands: []string{"valgrind"}},
			{Name: "ktlint", Commands: []string{"ktlint"}},
			{Name: "Detekt", Commands: []string{"detekt"}},
			{Name: "SwiftLint", Commands: []string{"swiftlint"}},
			{Name: "SwiftFormat", Commands: []string{"swiftformat"}},
			{Name: "PHPCS", Commands: []string{"phpcs"}},
			{Name: "PHPStan", Commands: []string{"phpstan"}},
			{Name: "Psalm", Commands: []string{"psalm"}},
			{Name: "hadolint", Commands: []string{"hadolint"}},
			{Name: "ShellCheck", Commands: []string{"shellcheck"}},
			{Name: "shfmt", Commands: []string{"shfmt"}},
			{Name: "yamllint", Commands: []string{"yamllint"}},
			{Name: "jsonlint", Commands: []string{"jsonlint"}},
			{Name: "markdownlint", Commands: []string{"markdownlint", "markdownlint-cli2"}},
			{Name: "Vale", Commands: []string{"vale"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// EDITORS & IDEs
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Editors & IDEs",
		Emoji: "📝",
		Tools: []ToolDef{
			{Name: "VS Code", Commands: []string{"code"}},
			{Name: "Cursor", Commands: []string{"cursor"}},
			{Name: "Zed", Commands: []string{"zed"}},
			{Name: "Neovim", Commands: []string{"nvim"}},
			{Name: "Vim", Commands: []string{"vim"}},
			{Name: "Emacs", Commands: []string{"emacs"}},
			{Name: "Nano", Commands: []string{"nano"}},
			{Name: "Helix", Commands: []string{"hx"}},
			// JetBrains IDEs: Toolbox shell scripts (idea, pycharm, …) on macOS/Linux
			// launch the full GUI instead of printing a version. Detect via native
			// OS registry / .app bundles / XDG .desktop files only.
			{
				Name: "IntelliJ IDEA",
				GUIApp: &GUIApp{
					WinExe:     "idea64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\IntelliJ IDEA*\bin`},
					MacApp:     "IntelliJ IDEA.app",
					LinuxBin:   "idea.sh",
					LinuxHints: []string{"/opt/idea*/bin", "/opt/intellij-idea*/bin"},
				},
			},
			{
				Name: "PyCharm",
				GUIApp: &GUIApp{
					WinExe:     "pycharm64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\PyCharm*\bin`},
					MacApp:     "PyCharm.app",
					LinuxBin:   "pycharm.sh",
					LinuxHints: []string{"/opt/pycharm*/bin"},
				},
			},
			{
				Name: "WebStorm",
				GUIApp: &GUIApp{
					WinExe:     "webstorm64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\WebStorm*\bin`},
					MacApp:     "WebStorm.app",
					LinuxBin:   "webstorm.sh",
					LinuxHints: []string{"/opt/webstorm*/bin"},
				},
			},
			{
				Name: "GoLand",
				GUIApp: &GUIApp{
					WinExe:     "goland64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\GoLand*\bin`},
					MacApp:     "GoLand.app",
					LinuxBin:   "goland.sh",
					LinuxHints: []string{"/opt/goland*/bin"},
				},
			},
			{
				Name: "CLion",
				GUIApp: &GUIApp{
					WinExe:     "clion64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\CLion*\bin`},
					MacApp:     "CLion.app",
					LinuxBin:   "clion.sh",
					LinuxHints: []string{"/opt/clion*/bin"},
				},
			},
			{
				Name: "Rider",
				GUIApp: &GUIApp{
					WinExe:     "rider64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\Rider*\bin`},
					MacApp:     "Rider.app",
					LinuxBin:   "rider.sh",
					LinuxHints: []string{"/opt/rider*/bin"},
				},
			},
			{
				Name: "RubyMine",
				GUIApp: &GUIApp{
					WinExe:     "rubymine64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\RubyMine*\bin`},
					MacApp:     "RubyMine.app",
					LinuxBin:   "rubymine.sh",
					LinuxHints: []string{"/opt/rubymine*/bin"},
				},
			},
			{
				Name: "PHPStorm",
				GUIApp: &GUIApp{
					WinExe:     "phpstorm64.exe",
					WinHints:   []string{`C:\Program Files\JetBrains\PhpStorm*\bin`},
					MacApp:     "PhpStorm.app",
					LinuxBin:   "phpstorm.sh",
					LinuxHints: []string{"/opt/phpstorm*/bin"},
				},
			},
			{
				Name: "Android Studio",
				GUIApp: &GUIApp{
					WinExe:     "studio64.exe",
					WinHints:   []string{`C:\Program Files\Android\Android Studio\bin`},
					MacApp:     "Android Studio.app",
					LinuxBin:   "studio.sh",
					LinuxHints: []string{"/opt/android-studio/bin"},
				},
			},
			{Name: "Eclipse", Commands: []string{"eclipse"}},
			{Name: "Qt Creator", Commands: []string{"qtcreator"}},
			{Name: "Sublime Text", Commands: []string{"subl", "sublime_text"}},
			{Name: "Geany", Commands: []string{"geany"}},
			{Name: "Kate", Commands: []string{"kate"}},
		{
			Name:     "RStudio",
			Commands: []string{"rstudio"},
			GUIApp: &GUIApp{
				WinExe:   "rstudio.exe",
				WinHints: []string{`C:\Program Files\RStudio`},
				MacApp:   "RStudio.app",
				LinuxBin: "rstudio",
				LinuxHints: []string{"/usr/lib/rstudio/bin", "/opt/rstudio/bin"},
			},
		},
		{
			Name:     "Visual Studio",
			Commands: []string{"devenv"},
			GUIApp: &GUIApp{
				WinExe:   "devenv.exe",
				WinHints: []string{
					`C:\Program Files\Microsoft Visual Studio\2*\Common7\IDE`,
					`C:\Program Files\Microsoft Visual Studio\9*\Common7\IDE`,
				},
			},
		},
		{
			Name:     "Notepad++",
			Commands: []string{"notepad++"},
			GUIApp: &GUIApp{
				WinExe:   "notepad++.exe",
				WinHints: []string{`C:\Program Files\Notepad++`, `C:\Program Files (x86)\Notepad++`},
			},
		},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// TERMINAL & SHELL TOOLS
	// (curl/wget removed — they live in Networking only)
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Terminal & Shell Tools",
		Emoji: "💻",
		Tools: []ToolDef{
			{Name: "tmux", Commands: []string{"tmux"}},
			{Name: "Screen", Commands: []string{"screen"}},
			{Name: "Zellij", Commands: []string{"zellij"}},
			{Name: "fzf", Commands: []string{"fzf"}},
			{Name: "ripgrep (rg)", Commands: []string{"rg"}},
			{Name: "fd", Commands: []string{"fd", "fdfind"}},
			{Name: "bat", Commands: []string{"bat", "batcat"}},
			{Name: "eza", Commands: []string{"eza"}},
			{Name: "lsd", Commands: []string{"lsd"}},
			{Name: "exa", Commands: []string{"exa"}},
			{Name: "jq", Commands: []string{"jq"}},
			{Name: "yq", Commands: []string{"yq"}},
			{Name: "fx", Commands: []string{"fx"}},
			{Name: "gron", Commands: []string{"gron"}},
			{Name: "xh", Commands: []string{"xh"}},
			{Name: "HTTPie", Commands: []string{"http", "httpie"}},
			{Name: "git-lfs", Commands: []string{"git-lfs"}},
			{Name: "delta", Commands: []string{"delta"}},
			{Name: "lazygit", Commands: []string{"lazygit"}},
			{Name: "tig", Commands: []string{"tig"}},
			{Name: "gitui", Commands: []string{"gitui"}},
			{Name: "lazydocker", Commands: []string{"lazydocker"}},
			{Name: "Starship", Commands: []string{"starship"}},
			{Name: "zoxide", Commands: []string{"zoxide"}},
			{Name: "autojump", Commands: []string{"autojump"}},
			{Name: "thefuck", Commands: []string{"thefuck"}},
			{Name: "tldr", Commands: []string{"tldr"}},
			{Name: "cheat", Commands: []string{"cheat"}},
			{Name: "htop", Commands: []string{"htop"}},
			{Name: "btop", Commands: []string{"btop"}},
			{Name: "glances", Commands: []string{"glances"}},
			{Name: "ctop", Commands: []string{"ctop"}},
			{Name: "ncdu", Commands: []string{"ncdu"}},
			{Name: "dust", Commands: []string{"dust"}},
			{Name: "duf", Commands: []string{"duf"}},
			{Name: "hyperfine", Commands: []string{"hyperfine"}},
			{Name: "tokei", Commands: []string{"tokei"}},
			{Name: "cloc", Commands: []string{"cloc"}},
			{Name: "mosh", Commands: []string{"mosh"}},
			{Name: "rsync", Commands: []string{"rsync"}},
			{Name: "rclone", Commands: []string{"rclone"}},
			{Name: "restic", Commands: []string{"restic"}},
			{Name: "aria2c", Commands: []string{"aria2c"}},
			{Name: "axel", Commands: []string{"axel"}},
			{Name: "gping", Commands: []string{"gping"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// API & TESTING TOOLS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "API & Testing Tools",
		Emoji: "🧪",
		Tools: []ToolDef{
			{Name: "k6", Commands: []string{"k6"}},
			{Name: "Artillery", Commands: []string{"artillery"}},
			{Name: "Locust", Commands: []string{"locust"}},
			// wrk and hey have no --version flag; detect by binary presence only.
			{Name: "wrk", Commands: []string{"wrk"}, NoVersion: true},
			{Name: "hey", Commands: []string{"hey"}, NoVersion: true},
			{Name: "vegeta", Commands: []string{"vegeta"}},
			{Name: "Playwright CLI", Commands: []string{"playwright"}},
			{Name: "Cypress", Commands: []string{"cypress"}},
			{Name: "Jest", Commands: []string{"jest"}},
			{Name: "Vitest", Commands: []string{"vitest"}},
			{Name: "Mocha", Commands: []string{"mocha"}},
			{Name: "pytest", Commands: []string{"pytest"}},
			{Name: "Newman", Commands: []string{"newman"}},
			{Name: "httpyac", Commands: []string{"httpyac"}},
			{Name: "JMeter", Commands: []string{"jmeter"}},
			// ab -v (lowercase) treats the arg as a verbosity level and needs a URL;
		// its output contains "HTTP/1.1" which causes "1.1" to be extracted.
		// -V (uppercase) prints the version header without requiring a URL.
		{Name: "Apache Bench (ab)", Commands: []string{"ab"}, VersionArgs: []string{"-V"}},
			{Name: "siege", Commands: []string{"siege"}},
			{Name: "gatling", Commands: []string{"gatling.sh", "gatling"}},
			{Name: "Bruno CLI", Commands: []string{"bru"}},
			// GUI API clients — detected via GUIApp; not on PATH by default.
			// Windows: both install per-user under %LOCALAPPDATA%\Programs\ (depth 2).
			{
				Name:     "Postman",
				Commands: []string{"postman"},
				GUIApp: &GUIApp{
					WinExe:   "Postman.exe",
					MacApp:   "Postman.app",
					LinuxBin: "postman",
					LinuxHints: []string{
						"/opt/Postman",
						"/usr/share/postman",
					},
				},
			},
			{
				Name:     "Insomnia",
				Commands: []string{"insomnia"},
				GUIApp: &GUIApp{
					WinExe:   "insomnia.exe",
					MacApp:   "Insomnia.app",
					LinuxBin: "insomnia",
					LinuxHints: []string{
						"/opt/Insomnia",
						"/usr/share/insomnia",
					},
				},
			},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// DATA, ML & AI TOOLS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Data, ML & AI Tools",
		Emoji: "🤖",
		Tools: []ToolDef{
			{Name: "Jupyter", Commands: []string{"jupyter"}, VersionArgs: []string{"--version"}, VersionFilter: "jupyter_core", Timeout: 10 * time.Second},
			{Name: "JupyterLab", Commands: []string{"jupyter"}, VersionArgs: []string{"lab --version"}},
			{Name: "IPython", Commands: []string{"ipython"}},
			// TensorBoard: VersionFilter avoids mistaking log timestamps for version numbers.
			{Name: "TensorBoard", Commands: []string{"tensorboard"}, VersionFilter: "TensorBoard"},
			{Name: "Spark (spark-submit)", Commands: []string{"spark-submit"}},
			{Name: "Ollama", Commands: []string{"ollama"}},
			{Name: "llamafile", Commands: []string{"llamafile"}},
			// huggingface-cli is Python-based; give it extra time to start up.
		// --version was added in v0.21; older installs need the "env" subcommand
		// which outputs "- huggingface_hub version: X.Y.Z" (semver3 matches fine).
		{Name: "Hugging Face CLI", Commands: []string{"huggingface-cli"}, VersionArgs: []string{"--version", "env"}, Timeout: 10 * time.Second},
			{Name: "dbt CLI", Commands: []string{"dbt"}},
			{Name: "Prefect CLI", Commands: []string{"prefect"}},
			{Name: "Airflow CLI", Commands: []string{"airflow"}},
			{Name: "Dagster CLI", Commands: []string{"dagster"}},
			{Name: "MLflow CLI", Commands: []string{"mlflow"}},
			{Name: "Weights & Biases (wandb)", Commands: []string{"wandb"}},
			{Name: "DVC", Commands: []string{"dvc"}},
			{Name: "Ray CLI", Commands: []string{"ray"}},
		// Hadoop: Deep heuristic — checks $HADOOP_HOME/bin/hadoop before file walker.
		// Also on PATH when $HADOOP_HOME/bin is exported (common on Linux clusters).
		{
			Name:          "Hadoop",
			Commands:      []string{"hadoop"},
			VersionArgs:   []string{"version"},
			VersionFilter: "Hadoop",
			GUIApp: &GUIApp{
				LinuxBin: "hadoop",
				LinuxHints: []string{"/usr/local/hadoop/bin", "/opt/hadoop*/bin"},
				EnvVar:   "HADOOP_HOME",
			},
		},
		{Name: "Databricks CLI", Commands: []string{"databricks"}, VersionArgs: []string{"--version", "-v", "version"}},
		{
			Name:     "LM Studio",
			Commands: []string{"lms"},
			GUIApp: &GUIApp{
				WinExe:   "LM Studio.exe",
				MacApp:   "LM Studio.app",
				LinuxBin: "lmstudio",
				LinuxHints: []string{"/opt/lmstudio"},
			},
		},
		{
			Name:     "GPT4All",
			Commands: []string{"gpt4all"},
			GUIApp: &GUIApp{
				WinExe:   "GPT4All.exe",
				MacApp:   "GPT4All.app",
				LinuxBin: "gpt4all",
			},
		},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// MOBILE & CROSS-PLATFORM
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Mobile & Cross-Platform",
		Emoji: "📱",
		Tools: []ToolDef{
			{Name: "Expo CLI", Commands: []string{"expo"}},
			{Name: "EAS CLI", Commands: []string{"eas"}},
			{Name: "ADB (Android)", Commands: []string{"adb"}},
			{Name: "avdmanager", Commands: []string{"avdmanager"}},
			// xcrun is a tool dispatcher; it has no version flag of its own.
		{Name: "xcrun", Commands: []string{"xcrun"}, NoVersion: true},
			{Name: "Ionic CLI", Commands: []string{"ionic"}},
			{Name: "Capacitor", Commands: []string{"cap"}},
			{Name: "Cordova", Commands: []string{"cordova"}},
			{Name: "Tauri CLI", Commands: []string{"tauri"}},
			{Name: "Electron", Commands: []string{"electron"}},
			{Name: "NativeScript", Commands: []string{"ns", "tns"}},
			{Name: "React Native CLI", Commands: []string{"react-native"}},
		{Name: "fastlane", Commands: []string{"fastlane"}},
		{Name: "CocoaPods (pod)", Commands: []string{"pod"}},
		// Watchman: Meta's file-watching service used by React Native, Jest, and Metro bundler.
		// Installed via Homebrew on macOS, Chocolatey/scoop on Windows, or apt/brew on Linux.
		// "--version" outputs a date-based version string like "2024.10.07.00".
		{Name: "Watchman", Commands: []string{"watchman"}, VersionArgs: []string{"--version"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// WEB SERVERS & PROXIES
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Web Servers & Proxies",
		Emoji: "🌐",
		Tools: []ToolDef{
			{Name: "Nginx", Commands: []string{"nginx"}},
			{Name: "Apache (httpd)", Commands: []string{"httpd", "apache2"}},
			{Name: "Caddy", Commands: []string{"caddy"}},
			{Name: "Traefik", Commands: []string{"traefik"}},
			{Name: "HAProxy", Commands: []string{"haproxy"}},
			{Name: "Varnish", Commands: []string{"varnishd"}},
			{Name: "Squid", Commands: []string{"squid"}},
			{Name: "Lighttpd", Commands: []string{"lighttpd"}},
			{Name: "IIS (appcmd)", Commands: []string{"appcmd"}},
			// Apache Tomcat: primary discovery via Windows Uninstall registry
			// ("Apache Tomcat 10.1 Tomcat10" substring match → DisplayVersion).
			// Deep heuristic fallback: $CATALINA_HOME/bin/catalina.sh (any OS).
			// Linux file-walker covers /opt/tomcat*/bin and /usr/share/tomcat*/bin.
			{
				Name:          "Apache Tomcat",
				Commands:      []string{"catalina.sh", "catalina"},
				VersionArgs:   []string{"version"},
				VersionFilter: "Apache Tomcat",
				GUIApp: &GUIApp{
					WinExe:   "catalina.bat",
					WinHints: []string{`C:\Program Files\Apache Software Foundation`},
					LinuxBin: "catalina.sh",
					LinuxHints: []string{
						"/opt/tomcat/bin",
						"/opt/apache-tomcat*/bin",
						"/usr/share/tomcat*/bin",
						"/usr/local/tomcat/bin",
					},
					EnvVar: "CATALINA_HOME",
				},
			},
			// XAMPP: all-in-one LAMP stack; no CLI version flag — presence-only.
			{
				Name:      "XAMPP",
				NoVersion: true,
				GUIApp: &GUIApp{
					WinExe:   "xampp-control.exe",
					WinHints: []string{`C:\xampp`, `C:\xampp-portable`},
					MacApp:   "XAMPP.app",
					LinuxBin: "xampp",
					LinuxHints: []string{"/opt/lampp/bin"},
				},
			},
			{Name: "Gunicorn", Commands: []string{"gunicorn"}},
			{Name: "uWSGI", Commands: []string{"uwsgi"}},
			{Name: "Uvicorn", Commands: []string{"uvicorn"}},
			{Name: "Hypercorn", Commands: []string{"hypercorn"}},
			{Name: "Puma", Commands: []string{"puma"}},
			{Name: "Passenger", Commands: []string{"passenger"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// NETWORKING & PROTOCOLS
	// (curl/wget only here; OpenSSL only in Security)
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Networking & Protocols",
		Emoji: "🔌",
		Tools: []ToolDef{
			{Name: "curl", Commands: []string{"curl"}},
			{Name: "wget", Commands: []string{"wget"}},
			// ssh -V outputs "OpenSSH_9.8p1, LibreSSL 3.3.6" to stderr.
			// semver3 picks up LibreSSL's "3.3.6" before the OpenSSH version.
			// VersionRegex anchors on "OpenSSH_" to capture only the correct number.
			{Name: "SSH", Commands: []string{"ssh"}, VersionArgs: []string{"-V"}, VersionRegex: `OpenSSH_(\d+\.\d+p?\d*)`},
			{Name: "SCP", Commands: []string{"scp"}, NoVersion: true},
			{Name: "SFTP", Commands: []string{"sftp"}, NoVersion: true},
			{Name: "rsync", Commands: []string{"rsync"}},
			{Name: "ncat/nc", Commands: []string{"ncat", "nc"}},
			{Name: "socat", Commands: []string{"socat"}},
			// iperf3 --version: "iperf 3.13 (cJSON 1.7.15)" — for 2-component iperf
			// versions, semver3 grabs cJSON's "1.7.15" before the iperf version.
			// VersionRegex anchors on "iperf " to capture the correct number.
			{Name: "iperf3", Commands: []string{"iperf3"}, VersionRegex: `iperf (\d+\.\d+(?:\.\d+)?)`},
			{Name: "dig", Commands: []string{"dig"}},
			// nslookup reports DNS server IPs in its version output — presence-only.
			{Name: "nslookup", Commands: []string{"nslookup"}, NoVersion: true},
			{Name: "host", Commands: []string{"host"}},
			// whois has no standard version flag; presence-only detection.
		{Name: "whois", Commands: []string{"whois"}, NoVersion: true},
			{Name: "traceroute", Commands: []string{"traceroute", "tracert"}, NoVersion: true},
			// mtr --version outputs "mtr 0.95" — the 0.x major is excluded by semver2
			// (which requires major ≥ 1 to avoid timestamp fragments like "07.109").
			// VersionRegex explicitly captures the version number regardless of major.
			{Name: "mtr", Commands: []string{"mtr"}, VersionRegex: `mtr (\d+\.\d+(?:\.\d+)?)`},
			// wg --version outputs "wireguard-tools vX.Y.Z" (to stderr, captured by tryGetVersion).
		{Name: "WireGuard (wg)", Commands: []string{"wg"}, VersionArgs: []string{"--version"}, VersionRegex: `wireguard-tools v(\d+\.\d+\.\d+)`},
			{Name: "OpenVPN", Commands: []string{"openvpn"}},
			{Name: "frpc", Commands: []string{"frpc"}},
			{Name: "ngrok", Commands: []string{"ngrok"}},
			{Name: "cloudflared", Commands: []string{"cloudflared"}},
			{Name: "stunnel", Commands: []string{"stunnel"}},
		{Name: "Protoc (Protocol Buffers)", Commands: []string{"protoc"}},
		{Name: "grpcurl", Commands: []string{"grpcurl"}, VersionArgs: []string{"-version", "--version"}},
		{
			Name:     "Wireshark",
			Commands: []string{"wireshark"},
			GUIApp: &GUIApp{
				WinExe:   "Wireshark.exe",
				WinHints: []string{`C:\Program Files\Wireshark`},
				MacApp:   "Wireshark.app",
				LinuxBin: "wireshark",
			},
		},
		{
			Name:     "Tailscale",
			Commands: []string{"tailscale"},
			VersionArgs: []string{"version"},
			GUIApp: &GUIApp{
				WinExe: "tailscale.exe",
				MacApp: "Tailscale.app",
				LinuxBin: "tailscale",
			},
		},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// DOCUMENTATION TOOLS
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Documentation Tools",
		Emoji: "📚",
		Tools: []ToolDef{
			{Name: "Doxygen", Commands: []string{"doxygen"}},
			{Name: "Sphinx", Commands: []string{"sphinx-build"}},
			{Name: "MkDocs", Commands: []string{"mkdocs"}},
			{Name: "Docusaurus", Commands: []string{"docusaurus"}},
			{Name: "VitePress", Commands: []string{"vitepress"}},
			{Name: "Astro", Commands: []string{"astro"}},
			{Name: "Javadoc", Commands: []string{"javadoc"}},
			// godoc has no --version flag; use go version as a proxy (detect presence).
			{Name: "godoc", Commands: []string{"godoc"}, NoVersion: true},
			{Name: "Swagger CLI", Commands: []string{"swagger-codegen", "swagger"}},
			{Name: "Pandoc", Commands: []string{"pandoc"}},
			{Name: "Asciidoctor", Commands: []string{"asciidoctor"}},
			{Name: "mdBook", Commands: []string{"mdbook"}},
			{Name: "PlantUML", Commands: []string{"plantuml"}},
			{Name: "Graphviz (dot)", Commands: []string{"dot"}},
			{Name: "Mermaid CLI", Commands: []string{"mmdc"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// GAME DEVELOPMENT
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Game Development",
		Emoji: "🎮",
		Tools: []ToolDef{
			{Name: "Godot", Commands: []string{"godot", "godot4", "godot3"}},
			{Name: "Love2D", Commands: []string{"love"}},
			{Name: "vulkaninfo", Commands: []string{"vulkaninfo"}, VersionArgs: []string{"--summary"}, VersionFilter: "Instance Version"},
			// Unity Hub: no CLI version flag; version from registry / .app plist.
			{
				Name:     "Unity Hub",
				Commands: []string{"unityhub"},
				GUIApp: &GUIApp{
					WinExe:     "Unity Hub.exe",
					WinHints:   []string{`C:\Program Files\Unity Hub`},
					MacApp:     "Unity Hub.app",
					LinuxBin:   "unityhub",
					LinuxHints: []string{"/opt/unityhub/bin", "/usr/share/unity-hub/bin"},
				},
			},
			// Unreal Engine: no CLI version flag; version from registry / install dir.
			{
				Name:     "Unreal Engine",
				Commands: []string{"UE4Editor", "UE5Editor", "UnrealEditor"},
				GUIApp: &GUIApp{
					WinExe:   "UnrealEditor.exe",
					WinHints: []string{`C:\Program Files\Epic Games`},
					MacApp:   "Unreal Engine.app",
				},
			},
			{Name: "DevKitPro (pacman)", Commands: []string{"dkp-pacman"}},
		},
	},

	// ──────────────────────────────────────────────────────────────
	// BLOCKCHAIN & WEB3
	// ──────────────────────────────────────────────────────────────
	{
		Name:  "Blockchain & Web3",
		Emoji: "⛓️",
		Tools: []ToolDef{
			{Name: "Foundry (forge)", Commands: []string{"forge"}},
			{Name: "Foundry (cast)", Commands: []string{"cast"}},
			{Name: "Foundry (anvil)", Commands: []string{"anvil"}},
			{Name: "Hardhat", Commands: []string{"hardhat"}},
			{Name: "Truffle", Commands: []string{"truffle"}},
			{Name: "Ganache", Commands: []string{"ganache", "ganache-cli"}},
			{Name: "NEAR CLI", Commands: []string{"near"}},
			{Name: "Solana CLI", Commands: []string{"solana"}},
			{Name: "Anchor (Solana)", Commands: []string{"anchor"}},
			{Name: "Sui CLI", Commands: []string{"sui"}},
			{Name: "Aptos CLI", Commands: []string{"aptos"}},
			{Name: "StarkNet CLI", Commands: []string{"starknet"}},
			{Name: "Slither", Commands: []string{"slither"}},
		},
	},
}
