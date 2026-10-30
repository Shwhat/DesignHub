# DesignHub Frontend Decision

| | |
|---|---|
| Document | Frontend framework and editor selection |
| Version | 0.1 (draft for review) |
| Date | 2026-10-30 |
| Owner | Devon |
| Status | Proposed |
| Related | [SPEC.md](SPEC.md), [SCREENS.md](SCREENS.md) |

## Contents

1. Decision
2. Requirements
3. Framework options
4. Framework comparison
5. Editor options
6. Editor comparison
7. Supporting libraries
8. How the frontend is built and shipped
9. Risks
10. References

---

## 1. Decision

| Part | Choice |
|---|---|
| Framework | Svelte 5 with TypeScript |
| Build tool | Vite |
| Visual editing mode | Milkdown |
| Source, Split and Preview toggle modes | CodeMirror 6, with the same markdown renderer used for preview |
| Delivery | Static files built once and embedded in the Go binary. Nothing loaded from the internet at runtime. |

## 2. Requirements

The frontend must meet these requirements from SPEC.md and SCREENS.md:

| Requirement | Source |
|---|---|
| All fonts, scripts and styles bundled; nothing loaded from external servers at runtime | SPEC.md 2 |
| Four editing modes (Visual, Split, Source, Preview toggle) reading and writing the same markdown | SPEC.md 6.3 |
| Formatting toolbar, image and file paste or drag and drop, autosaved drafts in every mode | SPEC.md 6.3 |
| Live blocks in pages (Bill of Design, work list, item card) inserted from a menu or by typing "/" | SCREENS.md 10 |
| Drag and drop: board cards, BOM lines, page tree, admin lists | SCREENS.md 7, 9, 11, 15 |
| Updates from other users without reloading (boards, work item status) | SPEC.md 4.4 |
| Responsive layouts for desktop, tablet and phone | SCREENS.md 1 |
| Light and dark themes | SCREENS.md 1 |
| Open source licenses compatible with company use | Project rule |
| No Python in the product | Project rule |

## 3. Framework options

### 3.1 Svelte 5 (chosen)

A compiler: components are written in a small extension of HTML, CSS and JavaScript and compiled at build time into plain JavaScript with no framework runtime to ship. Version 5 is the current major version and is actively released.

| For | Against |
|---|---|
| Small bundles and fast load on phones, because there is no runtime library | Smaller ecosystem than React. Fewer ready-made components. |
| Least code per screen of the options considered. Components read close to plain HTML. | Version 5 changed how state is written ("runes"). Older examples online use the version 4 style. |
| Built-in transitions, stores and scoped CSS cover theming and layout without extra libraries | Fewer developers know it than React, which matters if someone else takes over the code |
| Few dependencies, so fewer third-party packages to audit and update | |
| Milkdown and CodeMirror both work with it directly | |

### 3.2 React

The most widely used UI library. Components are JavaScript functions that return markup.

| For | Against |
|---|---|
| Largest ecosystem: tables, drag and drop, editors, charts | Larger bundles; ships a runtime library |
| Most developers know it; easiest to hand over | More code per screen and more state management patterns to choose between |
| Every editor library considered has React bindings | Pulls in more dependencies, each of which must be kept updated |

### 3.3 Vue 3

A framework between React and Svelte in size and style, with single-file components.

| For | Against |
|---|---|
| Mature, well documented, good ecosystem | Ships a runtime, larger than Svelte output |
| Templates close to HTML | Two component styles (Options API and Composition API) in examples and libraries |

### 3.4 Plain JavaScript (the OpenSchem approach)

No framework. The interface is built with browser APIs directly, one file per area.

| For | Against |
|---|---|
| No framework dependency at all | DesignHub has about 15 screens sharing records, lists, live updates and permissions. Without a framework, keeping the screen in step with the data is written by hand everywhere. |
| Same style as OpenSchem | More code and more places for display bugs |

### 3.5 Server-rendered pages with htmx

The Go server renders HTML (as the prototype does) and htmx swaps page fragments on clicks and form posts.

| For | Against |
|---|---|
| Builds on the prototype's server-rendered pages | The editor, drag and drop, autosaved drafts and live board updates all need substantial JavaScript anyway, so the app would end up with two approaches |
| Very little JavaScript for simple screens | Phone and tablet interactions (board as list, comment panel, decision bar) are harder to keep smooth |

### 3.6 Others considered

| Option | Reason not chosen |
|---|---|
| SolidJS | Similar strengths to Svelte with a smaller ecosystem and community |
| Lit (web components) | Low-level; more code for application screens |
| Angular | Heavy for a single-team application; large bundles |

## 4. Framework comparison

| Criterion | Svelte 5 | React | Vue 3 | Plain JS | htmx |
|---|---|---|---|---|---|
| Bundle size on phone | Smallest | Largest | Medium | Smallest | Small, but editor JS added |
| Code per screen | Low | High | Medium | Highest | Low for simple screens, high for interactive ones |
| Live updates and drag and drop | Good | Good | Good | Hand-written | Awkward |
| Ecosystem | Medium | Largest | Large | None needed | Small |
| Ease of handover | Medium | Easiest | Good | Hard | Medium |
| Dependencies to maintain | Few | Many | Medium | None | Few |

## 5. Editor options

No single library provides all four modes with clean markdown in and out. The chosen approach combines two libraries over one markdown source:

| Mode | Implementation |
|---|---|
| Visual | Milkdown. Edits a document tree and writes markdown on every change. |
| Source | CodeMirror 6 with markdown highlighting |
| Split | CodeMirror 6 on the left (above on phone), rendered preview on the right |
| Preview toggle | CodeMirror 6 with a button that swaps to the rendered view |

Switching modes passes the current markdown text from one editor to the other. The preview uses the same renderer as the page view, so preview and saved page always match.

### 5.1 Milkdown (chosen for Visual)

A WYSIWYG markdown editor built on ProseMirror (document editing engine) and remark (markdown parser). MIT license. Very active, with frequent releases.

| For | Against |
|---|---|
| Markdown is its native format, not an export | Rewrites markdown into a consistent style (for example list markers and emphasis characters) the first time a page is edited in Visual mode |
| Plugin system for toolbar, slash menu, tables and custom blocks such as live blocks | Heaviest of the editor options because of ProseMirror |
| Works with any framework, including Svelte | |

### 5.2 CodeMirror 6 (chosen for Source, Split and Preview toggle)

The standard browser code editor. MIT license. Mature and maintained.

| For | Against |
|---|---|
| Edits the markdown text exactly as stored, with no reformatting | Not a WYSIWYG editor, so it cannot provide Visual mode |
| Fast on long documents and works well on phones | |
| Markdown highlighting, search and keyboard shortcuts built in | |

### 5.3 Vditor

A single editor with WYSIWYG, instant rendering and split modes. MIT license. Active.

| For | Against |
|---|---|
| Covers three of the four modes in one library | Loads parts of itself from a CDN by default; must be reconfigured and checked to run fully offline |
| | Uses its own markdown parser (Lute), so its rendering can differ from the page view |
| | Mostly Chinese-language documentation and community |

### 5.4 Tiptap

A rich-text editor built on ProseMirror with a large extension library.

| For | Against |
|---|---|
| Polished editing experience and many extensions | HTML-first. Markdown support comes from separate extensions, so markdown fidelity is weaker. |
| | Some features are paid add-ons |

### 5.5 ByteMD

A small Svelte-built markdown editor with split preview.

| For | Against |
|---|---|
| Very small | Version 1 is frozen; its successor (HashMD) is still in development |
| | No true Visual mode |

### 5.6 TOAST UI Editor

A markdown editor with markdown and WYSIWYG tabs.

| For | Against |
|---|---|
| Two modes in one library | Repository archived in September 2026; no further maintenance |

## 6. Editor comparison

| Criterion | Milkdown + CodeMirror | Vditor | Tiptap | ByteMD | TOAST UI |
|---|---|---|---|---|---|
| All four modes | Yes | Three of four | Visual only | Split and preview only | Two of four |
| Markdown is the stored format | Yes | Yes | Through extensions | Yes | Yes |
| Preview matches page view | Yes, one renderer | No, own parser | Not applicable | Configurable | No |
| Fully offline by default | Yes | No | Yes | Yes | Yes |
| Maintained | Yes | Yes | Yes | Frozen | Archived |
| License | MIT | MIT | MIT core, paid extras | MIT | MIT |

## 7. Supporting libraries

| Need | Library | License |
|---|---|---|
| Drag and drop (board, BOM lines, page tree, lists) | SortableJS | MIT |
| Markdown rendering for preview and page view | remark and rehype, matching Milkdown's parser | MIT |
| HTML sanitizing of rendered markdown | DOMPurify | Apache 2.0 or MPL 2.0 |
| Icons | A bundled open icon set (for example Lucide) | ISC |
| Fonts | Bundled open fonts (for example Inter) | SIL Open Font License |

Live updates from other users use Server-Sent Events from the Go server. This is a browser feature and needs no library.

## 8. How the frontend is built and shipped

1. Source lives in `web/` in the repository.
2. Vite compiles the Svelte components and bundles all libraries, fonts and icons into static files.
3. The Go build embeds those files into the `designhub` binary.
4. GitHub Actions runs both builds and publishes the image to `ghcr.io/shwhat/designhub`.
5. At runtime the browser loads everything from the DesignHub server only. Node.js and npm are used at build time only and are not in the image.

## 9. Risks

| Risk | Effect | Response |
|---|---|---|
| Milkdown normalizes markdown formatting on first Visual edit | A page's markdown text changes style without a content change | Normalization is applied consistently on save in every mode, so history diffs show only content changes after the first save |
| Svelte ecosystem smaller than React | Some components are built rather than installed | Supporting libraries in section 7 are framework-independent |
| Two editor libraries | Two sets of behavior to keep consistent | Shared toolbar commands and one markdown renderer |
| Dependency updates | Security fixes in npm packages | Locked versions, reviewed updates, no runtime loading |

## 10. References

- [Svelte releases](https://releases.sh/svelte/svelte-releases.md)
- [Markdown editor components compared: Milkdown, Vditor, ByteMD](https://www.pistack.xyz/posts/2026-09-10-markdown-editor-components-milkdown-vditor-bytemd-comparison/)
- [TOAST UI Editor repository status](https://github.com/nhn/tui.editor/issues/3229)
- [tiptap-markdown package](https://npmjs.com/package/tiptap-markdown)
