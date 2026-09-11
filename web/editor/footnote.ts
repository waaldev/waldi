import { Mark, mergeAttributes } from '@tiptap/core'
import type { Node as PMNode } from '@tiptap/pm/model'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

export type FootnoteContent = Array<{ type: string; content?: Array<{ text?: string }> }>

export interface FootnoteRange {
  from: number
  to: number
}

export interface FootnoteRun extends FootnoteRange {
  id: string
  attrs: Record<string, unknown>
}

interface FootnotePluginState {
  active: FootnoteRange | null
  decorations: DecorationSet
}

export const footnotePluginKey = new PluginKey<FootnotePluginState>('footnote')

export function footnoteRuns(doc: PMNode): FootnoteRun[] {
  const runs: FootnoteRun[] = []
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true
    let current: FootnoteRun | null = null
    let offset = pos + 1
    for (let i = 0; i < node.childCount; i++) {
      const child = node.child(i)
      const mark = child.marks.find((m) => m.type.name === 'footnote')
      const id = mark ? String(mark.attrs.id || '') : ''
      const end = offset + child.nodeSize
      if (current && current.id === id) {
        current.to = end
      } else if (mark && id) {
        current = { id, from: offset, to: end, attrs: mark.attrs }
        runs.push(current)
      } else {
        current = null
      }
      offset = end
    }
    return false
  })
  return runs
}

export function footnoteNumbers(runs: FootnoteRun[]): Map<string, number> {
  const numbers = new Map<string, number>()
  for (const run of runs) {
    if (!numbers.has(run.id)) numbers.set(run.id, numbers.size + 1)
  }
  return numbers
}

export function footnoteNumberAt(runs: FootnoteRun[], pos: number): number {
  return footnoteNumbers(runs.filter((run) => run.from < pos)).size + 1
}

export function nextFootnoteId(runs: FootnoteRun[]): string {
  let max = 0
  for (const run of runs) {
    const match = /^fn(\d+)$/.exec(run.id)
    if (match) max = Math.max(max, Number(match[1]))
  }
  return `fn${max + 1}`
}

export function footnoteText(content: FootnoteContent): string {
  return content
    .map((paragraph) => (paragraph.content ?? []).map((node) => node.text ?? '').join('').trim())
    .filter(Boolean)
    .join('\n')
}

export function footnoteContentFromAttrs(attrs: Record<string, unknown>): FootnoteContent {
  if (Array.isArray(attrs.content) && attrs.content.length > 0) {
    return attrs.content as FootnoteContent
  }
  return String(attrs.text || '')
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => ({ type: 'paragraph', content: [{ type: 'text', text: line }] }))
}

function parseContent(raw: string | null): FootnoteContent | null {
  if (!raw) return null
  try {
    const value = JSON.parse(raw)
    return Array.isArray(value) ? value : null
  } catch {
    return null
  }
}

function numberWidget(pos: number, id: string, num: number) {
  return Decoration.widget(
    pos,
    () => {
      const marker = document.createElement('sup')
      marker.className = 'fn-ref fn-num'
      marker.contentEditable = 'false'
      marker.dataset.fnId = id
      marker.textContent = String(num)
      return marker
    },
    { side: -1, key: `fn-${id}-${num}`, ignoreSelection: true },
  )
}

function buildDecorations(doc: PMNode, active: FootnoteRange | null): DecorationSet {
  const runs = footnoteRuns(doc)
  const numbers = footnoteNumbers(runs)
  const decorations = runs.map((run) => numberWidget(run.to, run.id, numbers.get(run.id) ?? 0))
  if (active && active.from < active.to) {
    decorations.push(Decoration.inline(active.from, active.to, { class: 'fn-editing' }))
  }
  return DecorationSet.create(doc, decorations)
}

export const Footnote = Mark.create({
  name: 'footnote',
  priority: 1001,
  inclusive: false,
  excludes: 'link footnote',
  addAttributes() {
    return {
      id: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-fn-id'),
        renderHTML: (attributes) =>
          attributes.id ? { 'data-fn-id': attributes.id } : {},
      },
      text: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-fn-text'),
        renderHTML: (attributes) =>
          attributes.text ? { 'data-fn-text': attributes.text } : {},
      },
      content: {
        default: null,
        parseHTML: (element) => parseContent(element.getAttribute('data-fn-content')),
        renderHTML: (attributes) =>
          Array.isArray(attributes.content) && attributes.content.length > 0
            ? { 'data-fn-content': JSON.stringify(attributes.content) }
            : {},
      },
    }
  },
  parseHTML() {
    return [{ tag: 'span[data-fn-id]' }, { tag: 'sup.fn-ref[data-fn-id]' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes({ class: 'fn-anchor' }, HTMLAttributes), 0]
  },
  addProseMirrorPlugins() {
    return [
      new Plugin<FootnotePluginState>({
        key: footnotePluginKey,
        state: {
          init: (_, state) => ({ active: null, decorations: buildDecorations(state.doc, null) }),
          apply: (tr, prev) => {
            const meta = tr.getMeta(footnotePluginKey) as { active: FootnoteRange | null } | undefined
            if (!meta && !tr.docChanged) return prev
            let active = meta ? meta.active : prev.active
            if (!meta && active) {
              const from = tr.mapping.map(active.from, 1)
              const to = tr.mapping.map(active.to, -1)
              active = from < to ? { from, to } : null
            }
            return { active, decorations: buildDecorations(tr.doc, active) }
          },
        },
        props: {
          decorations: (state) => footnotePluginKey.getState(state)?.decorations,
        },
      }),
    ]
  },
})
