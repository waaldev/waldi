import { autoUpdate, computePosition, flip, offset, shift } from '@floating-ui/dom'
import { Editor, Extension, posToDOMRect } from '@tiptap/core'
import Bold from '@tiptap/extension-bold'
import Document from '@tiptap/extension-document'
import History from '@tiptap/extension-history'
import Italic from '@tiptap/extension-italic'
import Link from '@tiptap/extension-link'
import Paragraph from '@tiptap/extension-paragraph'
import Text from '@tiptap/extension-text'
import Typography from '@tiptap/extension-typography'
import { Placeholder } from '@tiptap/extensions'
import { TextSelection } from '@tiptap/pm/state'

import {
  type FootnoteContent,
  type FootnoteRange,
  footnoteContentFromAttrs,
  footnoteNumberAt,
  footnoteNumbers,
  footnotePluginKey,
  footnoteRuns,
  footnoteText,
  nextFootnoteId,
} from './footnote'

interface FootnotePopoverOptions {
  lang: string
  dir: string
  title: string
  placeholder: string
  formatNumber: (value: number) => string
}

interface Session {
  id: string
  existing: boolean
  initial: string
  note: Editor
  stopPositioning: () => void
}

export interface FootnotePopover {
  openForSelection: () => void
  openAt: (id: string, pos: number) => void
}

const TOOL_MARKS = ['bold', 'italic', 'link']

function cleanContent(content: unknown): FootnoteContent {
  if (!Array.isArray(content)) return []
  return (content as FootnoteContent).filter((paragraph) => footnoteText([paragraph]) !== '')
}

function normalizeHref(value: string): string {
  const href = value.trim()
  if (href === '' || /^[a-z][a-z0-9+.-]*:/i.test(href)) return href
  return `https://${href}`
}

export function createFootnotePopover(
  editor: Editor,
  popover: HTMLElement,
  options: FootnotePopoverOptions,
): FootnotePopover | null {
  const mount = popover.querySelector<HTMLElement>('[data-fn-mount]')
  const title = popover.querySelector<HTMLElement>('[data-fn-title]')
  const linkRow = popover.querySelector<HTMLElement>('[data-fn-link]')
  const linkInput = popover.querySelector<HTMLInputElement>('[data-fn-link-input]')
  const removeButton = popover.querySelector<HTMLButtonElement>('[data-fn-command="remove"]')
  if (!mount || !title || !linkRow || !linkInput || !removeButton) return null

  let session: Session | null = null

  const activeRange = (): FootnoteRange | null =>
    footnotePluginKey.getState(editor.state)?.active ?? null

  const reference = {
    contextElement: editor.view.dom,
    getBoundingClientRect: () => {
      const range = activeRange()
      return range
        ? posToDOMRect(editor.view, range.from, range.to)
        : editor.view.dom.getBoundingClientRect()
    },
  }

  const docked = window.matchMedia('(max-width: 600px)')

  const position = () => {
    popover.classList.toggle('is-docked', docked.matches)
    if (docked.matches) {
      const viewport = window.visualViewport
      const keyboard = viewport ? Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) : 0
      popover.style.removeProperty('left')
      popover.style.removeProperty('top')
      popover.style.setProperty('--fn-dock-offset', `${keyboard}px`)
      return
    }
    void computePosition(reference, popover, {
      placement: 'bottom',
      middleware: [offset(10), flip({ padding: 8 }), shift({ padding: 8 })],
    }).then(({ x, y }) => {
      popover.style.left = `${x}px`
      popover.style.top = `${y}px`
    })
  }

  const updateTools = () => {
    const note = session?.note
    for (const mark of TOOL_MARKS) {
      const button = popover.querySelector<HTMLButtonElement>(`[data-fn-command="${mark}"]`)
      button?.classList.toggle('is-active', Boolean(note?.isActive(mark)))
    }
  }

  const showLinkRow = () => {
    if (!session) return
    linkInput.value = String(session.note.getAttributes('link').href ?? '')
    linkRow.hidden = false
    linkInput.focus()
    linkInput.select()
  }

  const hideLinkRow = () => {
    linkRow.hidden = true
    session?.note.commands.focus()
  }

  const applyLink = () => {
    if (!session) return
    const { note } = session
    const href = normalizeHref(linkInput.value)
    linkRow.hidden = true
    if (href === '') {
      note.chain().focus().extendMarkRange('link').unsetLink().run()
      return
    }
    if (note.state.selection.empty && !note.isActive('link')) {
      note
        .chain()
        .focus()
        .insertContent({ type: 'text', text: href, marks: [{ type: 'link', attrs: { href } }] })
        .run()
      return
    }
    note.chain().focus().extendMarkRange('link').setLink({ href }).run()
  }

  const finish = (action: 'save' | 'remove', restoreFocus: boolean) => {
    if (!session) return
    const { id, existing, initial, note, stopPositioning } = session
    session = null
    const range = activeRange()
    const content = action === 'remove' ? [] : cleanContent(note.getJSON().content)

    stopPositioning()
    popover.hidden = true
    linkRow.hidden = true
    mount.replaceChildren()
    window.setTimeout(() => note.destroy(), 0)

    const type = editor.schema.marks.footnote
    const tr = editor.state.tr.setMeta(footnotePluginKey, { active: null })
    if (range && JSON.stringify(content) !== initial) {
      if (content.length > 0) {
        tr.addMark(range.from, range.to, type.create({ id, text: footnoteText(content), content }))
      } else if (existing) {
        tr.removeMark(range.from, range.to, type)
      }
    }
    if (!tr.docChanged) tr.setMeta('addToHistory', false)
    if (restoreFocus && range) tr.setSelection(TextSelection.create(tr.doc, range.to))
    editor.view.dispatch(tr)
    if (restoreFocus) editor.view.focus()
  }

  const open = (range: FootnoteRange, id: string, content: FootnoteContent, existing: boolean, num: number) => {
    const host = document.createElement('div')
    mount.replaceChildren(host)

    const note = new Editor({
      element: host,
      content: { type: 'doc', content: content.length > 0 ? content : [{ type: 'paragraph' }] },
      editorProps: {
        attributes: { class: 'editor-surface fn-note', dir: options.dir, lang: options.lang },
      },
      extensions: [
        Document,
        Paragraph,
        Text,
        Bold,
        Italic,
        History,
        Typography,
        Link.configure({
          openOnClick: false,
          autolink: true,
          defaultProtocol: 'https',
          protocols: ['http', 'https', 'mailto'],
        }),
        Placeholder.configure({ placeholder: options.placeholder }),
        Extension.create({
          name: 'footnoteKeys',
          priority: 1000,
          addKeyboardShortcuts: () => ({
            'Mod-Enter': () => {
              finish('save', true)
              return true
            },
            Escape: () => {
              finish('save', true)
              return true
            },
            'Mod-k': () => {
              showLinkRow()
              return true
            },
          }),
        }),
      ],
      onTransaction: () => updateTools(),
    })

    session = { id, existing, initial: JSON.stringify(content), note, stopPositioning: () => {} }
    title.textContent = `${options.title} ${options.formatNumber(num)}`
    removeButton.hidden = !existing
    linkRow.hidden = true
    popover.hidden = false
    note.commands.focus('end')
    editor.view.dispatch(
      editor.state.tr
        .setMeta(footnotePluginKey, { active: { from: range.from, to: range.to } })
        .setMeta('addToHistory', false),
    )
    const stopAutoUpdate = autoUpdate(reference, popover, position)
    const viewport = window.visualViewport
    viewport?.addEventListener('resize', position)
    viewport?.addEventListener('scroll', position)
    session.stopPositioning = () => {
      stopAutoUpdate()
      viewport?.removeEventListener('resize', position)
      viewport?.removeEventListener('scroll', position)
    }
    updateTools()
  }

  const openForSelection = () => {
    finish('save', false)
    const { selection, doc } = editor.state
    const runs = footnoteRuns(doc)
    const current = runs.find((run) => run.from < selection.to && selection.from < run.to)
    if (current) {
      const num = footnoteNumbers(runs).get(current.id) ?? 1
      open(current, current.id, footnoteContentFromAttrs(current.attrs), true, num)
      return
    }
    const { from, $from } = selection
    const to = Math.min(selection.to, $from.end())
    if (!$from.parent.isTextblock || from >= to) return
    open({ from, to }, nextFootnoteId(runs), [], false, footnoteNumberAt(runs, from))
  }

  const openAt = (id: string, pos: number) => {
    finish('save', false)
    const runs = footnoteRuns(editor.state.doc)
    const run =
      runs.find((candidate) => candidate.id === id && candidate.from <= pos && pos <= candidate.to) ??
      runs.find((candidate) => candidate.id === id)
    if (!run) return
    open(run, id, footnoteContentFromAttrs(run.attrs), true, footnoteNumbers(runs).get(id) ?? 1)
  }

  popover.addEventListener('mousedown', (event) => {
    if ((event.target as HTMLElement).closest('button')) event.preventDefault()
  })

  popover.addEventListener('click', (event) => {
    const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-fn-command]')
    if (!button || !session) return
    event.preventDefault()
    const { note } = session
    switch (button.dataset.fnCommand) {
      case 'bold':
        note.chain().focus().toggleBold().run()
        break
      case 'italic':
        note.chain().focus().toggleItalic().run()
        break
      case 'link':
        if (linkRow.hidden) showLinkRow()
        else hideLinkRow()
        break
      case 'link-apply':
        applyLink()
        break
      case 'done':
        finish('save', true)
        break
      case 'remove':
        finish('remove', true)
        break
    }
  })

  linkInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      event.preventDefault()
      applyLink()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      hideLinkRow()
    }
  })

  document.addEventListener(
    'pointerdown',
    (event) => {
      if (!session || popover.contains(event.target as Node)) return
      finish('save', false)
    },
    true,
  )

  return { openForSelection, openAt }
}
