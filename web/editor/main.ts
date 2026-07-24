import { Editor, isTextSelection } from '@tiptap/core'
import Blockquote from '@tiptap/extension-blockquote'
import Bold from '@tiptap/extension-bold'
import { BubbleMenu } from '@tiptap/extension-bubble-menu'
import Document from '@tiptap/extension-document'
import Dropcursor from '@tiptap/extension-dropcursor'
import Gapcursor from '@tiptap/extension-gapcursor'
import Heading from '@tiptap/extension-heading'
import History from '@tiptap/extension-history'
import HorizontalRule from '@tiptap/extension-horizontal-rule'
import Image from '@tiptap/extension-image'
import Italic from '@tiptap/extension-italic'
import Link from '@tiptap/extension-link'
import { BulletList, ListItem, ListKeymap, OrderedList } from '@tiptap/extension-list'
import Paragraph from '@tiptap/extension-paragraph'
import Text from '@tiptap/extension-text'
import Typography from '@tiptap/extension-typography'
import { NodeSelection } from '@tiptap/pm/state'

import { Aside } from './aside'
import { Direction, DirectionMark } from './direction'
import { Embed, parseEmbedURL } from './embed'
import { Footnote } from './footnote'

type SaveState = 'idle' | 'saving' | 'saved' | 'error'

const ImageBubbleMenu = BubbleMenu.extend({ name: 'imageBubbleMenu' })
const EmbedBubbleMenu = BubbleMenu.extend({ name: 'embedBubbleMenu' })

function focusParagraphAfterBlock(editor: Editor) {
  const { state } = editor
  const { selection, doc } = state

  const afterPos = selection instanceof NodeSelection
    ? selection.to
    : selection.$to.after()

  const nodeAfter = doc.nodeAt(afterPos)
  if (nodeAfter?.type.name === 'paragraph') {
    editor.chain().focus().setTextSelection(afterPos + 1).run()
    return
  }

  editor.chain().focus().insertContentAt(afterPos, { type: 'paragraph' }).run()
}

const ATOM_BLOCK_TYPES = new Set(['image', 'embed'])

function ensureParagraphsAfterImages(editor: Editor) {
  const { doc } = editor.state
  const insertAt: number[] = []

  let pos = 0
  for (let i = 0; i < doc.childCount; i++) {
    const node = doc.child(i)
    if (ATOM_BLOCK_TYPES.has(node.type.name) && i === doc.childCount - 1) {
      insertAt.push(pos + node.nodeSize)
    }
    pos += node.nodeSize
  }

  if (insertAt.length === 0) return

  let chain = editor.chain()
  for (let i = insertAt.length - 1; i >= 0; i--) {
    chain = chain.insertContentAt(insertAt[i], { type: 'paragraph' })
  }
  chain.run()
}

function uiDigits(lang: string, value: string): string {
  if (lang !== 'fa') return value
  return value.replace(/[0-9]/g, (d) => String.fromCodePoint(0x06f0 + Number(d)))
}

const root = document.querySelector<HTMLElement>('[data-editor-root]')

if (root) {
  const lang = root.dataset.lang === 'en' ? 'en' : 'fa'
  const pageLang = root.dataset.pageLang === 'en' ? 'en' : 'fa'
  const dir = root.dataset.dir === 'ltr' ? 'ltr' : 'rtl'
  const ui = {
    saving: root.dataset.uiSaving || 'Saving',
    saved: root.dataset.uiSaved || 'Saved',
    error: root.dataset.uiError || 'Save failed',
    linkPrompt: root.dataset.uiLinkPrompt || 'Link URL',
    footnotePrompt: root.dataset.uiFootnotePrompt || 'Footnote text',
    embedPrompt: root.dataset.uiEmbedPrompt || 'YouTube, Spotify, or SoundCloud URL',
    embedInvalid: root.dataset.uiEmbedInvalid || "That URL isn't a supported embed",
    words: root.dataset.uiWords || '%d words',
  }

  const form = root.querySelector<HTMLFormElement>('[data-editor-form]')
  const titleInput = root.querySelector<HTMLInputElement>('[data-editor-title]')
  const docJSON = root.querySelector<HTMLScriptElement>('[data-editor-doc-json]')
  const docInput = root.querySelector<HTMLTextAreaElement>('[data-editor-doc]')
  const canvas = root.querySelector<HTMLElement>('[data-editor-canvas]')
  const mount = root.querySelector<HTMLElement>('[data-editor-mount]')
  const status = document.querySelector<HTMLElement>('[data-editor-status]')
  const bubbleMenu = root.querySelector<HTMLElement>('[data-bubble-menu]')
  const imageMenu = root.querySelector<HTMLElement>('[data-image-menu]')
  const embedMenu = root.querySelector<HTMLElement>('[data-embed-menu]')
  const plusControl = root.querySelector<HTMLElement>('[data-plus-control]')
  const plusButton = root.querySelector<HTMLButtonElement>('[data-plus-button]')
  const plusMenu = root.querySelector<HTMLElement>('[data-plus-menu]')
  const imageInput = root.querySelector<HTMLInputElement>('[data-editor-image]')
  const publishForm = root.querySelector<HTMLFormElement>('[data-publish-form]')
  const draftForm = root.querySelector<HTMLFormElement>('[data-draft-form]')
  const wordCount = root.querySelector<HTMLElement>('[data-editor-words]')

  if (
    form && titleInput && docJSON && docInput && canvas && mount && status &&
    bubbleMenu && imageMenu && embedMenu && plusControl && plusButton && plusMenu && imageInput &&
    (publishForm || draftForm) &&
    form.dataset.saveUrl
  ) {
    const saveURL = form.dataset.saveUrl
    const rawDoc = docJSON.textContent?.trim() ?? docInput.value.trim()

    let initialContent: unknown
    let docLoadFailed = false
    let expectedWords = 0
    try {
      initialContent = JSON.parse(rawDoc)
      expectedWords = countDocWords(initialContent)
    } catch {
      docLoadFailed = true
      initialContent = { type: 'doc', content: [{ type: 'paragraph' }] }
    }

    let editorReady = false

    const editor = new Editor({
      element: mount,
      content: initialContent,
      autofocus: 'end',
      editorProps: {
        attributes: {
          class: 'editor-surface',
          dir,
          lang,
        },
      },
      extensions: [
        Document,
        Paragraph,
        Text,
        Bold,
        Italic,
        Aside,
        Blockquote,
        BulletList,
        OrderedList,
        ListItem,
        ListKeymap,
        HorizontalRule,
        Dropcursor,
        Gapcursor,
        History,
        Typography,
        Image.configure({
          allowBase64: false,
          inline: false,
        }),
        Embed,
        Heading.configure({ levels: [2] }),
        Link.configure({
          openOnClick: false,
          autolink: true,
          defaultProtocol: 'https',
          protocols: ['http', 'https', 'mailto'],
        }),
        Footnote,
        Direction,
        DirectionMark,
        BubbleMenu.configure({
          element: bubbleMenu,
          pluginKey: 'textBubbleMenu',
          options: { placement: 'top', offset: 10 },
          shouldShow: ({ editor, state, from, to }) => {
            const { selection } = state
            if (selection instanceof NodeSelection) return false
            if (!isTextSelection(selection) || selection.empty) return false
            if (!state.doc.textBetween(from, to).length) return false
            const isChildOfMenu = bubbleMenu.contains(document.activeElement)
            const hasEditorFocus = editor.view.hasFocus() || isChildOfMenu
            return hasEditorFocus && editor.isEditable
          },
        }),
        ImageBubbleMenu.configure({
          element: imageMenu,
          pluginKey: 'imageBubbleMenu',
          options: { placement: 'top', offset: 10 },
          shouldShow: ({ state }) => {
            const { selection } = state
            return selection instanceof NodeSelection && selection.node.type.name === 'image'
          },
        }),
        EmbedBubbleMenu.configure({
          element: embedMenu,
          pluginKey: 'embedBubbleMenu',
          options: { placement: 'top', offset: 10 },
          shouldShow: ({ state }) => {
            const { selection } = state
            return selection instanceof NodeSelection && selection.node.type.name === 'embed'
          },
        }),
      ],
      onSelectionUpdate: () => {
        updatePlus()
        updateBubbleActiveState()
      },
      onUpdate: () => {
        syncDoc()
        updateWordCount()
        updatePlus()
        updateBubbleActiveState()
        if (editorReady && !docLoadFailed) {
          scheduleSave()
        }
      },
      onFocus: () => updatePlus(),
      onBlur: () => {
        if (!plusInteracting) closePlus()
      },
      onCreate: ({ editor: created }) => {
        ensureParagraphsAfterImages(created)
        editorReady = true
        if (docLoadFailed) {
          setState('error')
          return
        }
        const words = created.getText().trim().split(/\s+/).filter(Boolean).length
        if (expectedWords > 0 && words === 0) {
          docLoadFailed = true
          setState('error')
          return
        }
        syncDoc()
        updateWordCount()
      },
    })

    const updateWordCount = () => {
      if (!wordCount) return
      const words = editor.getText().trim().split(/\s+/).filter(Boolean).length
      wordCount.textContent = uiDigits(pageLang, ui.words.replace('%d', String(words)))
    }

    const setState = (state: SaveState) => {
      const labels: Record<SaveState, string> = {
        idle: '',
        saving: ui.saving,
        saved: ui.saved,
        error: ui.error,
      }
      status.textContent = labels[state]
      status.dataset.state = state
    }

    const syncDoc = () => {
      docInput.value = JSON.stringify(editor.getJSON())
    }

    const runCommand = (command: string) => {
      switch (command) {
        case 'bold':
          editor.chain().focus().toggleBold().run()
          break
        case 'italic':
          editor.chain().focus().toggleItalic().run()
          break
        case 'heading':
          editor.chain().focus().toggleHeading({ level: 2 }).run()
          break
        case 'quote':
          editor.chain().focus().toggleBlockquote().run()
          break
        case 'aside':
          editor.chain().focus().toggleAside().run()
          break
        case 'bullet-list':
          editor.chain().focus().toggleBulletList().run()
          break
        case 'ordered-list':
          editor.chain().focus().toggleOrderedList().run()
          break
        case 'divider':
          editor.chain().focus().setHorizontalRule().run()
          break
        case 'image':
          imageInput.click()
          break
        case 'link':
          setLink(editor, ui.linkPrompt)
          break
        case 'footnote':
          setFootnote(editor, ui.footnotePrompt)
          break
        case 'ltr':
        case 'rtl':
          toggleDirection(editor, command)
          break
        case 'embed':
          setEmbed(editor, ui.embedPrompt, ui.embedInvalid)
          break
        case 'delete-image':
        case 'delete-embed':
          editor.chain().focus().deleteSelection().run()
          break
      }
      syncDoc()
      scheduleSave()
    }

    // ---------- floating "+" control for empty lines ----------

    let plusInteracting = false

    const closePlus = () => {
      plusControl.classList.remove('is-visible', 'menu-open')
    }

    const closePlusMenu = () => {
      plusControl.classList.remove('menu-open')
    }

    const updatePlus = () => {
      if (plusControl.classList.contains('menu-open')) return
      const { state, isFocused } = editor
      const { selection } = state
      const { $from } = selection

      const isEmptyTopLevelParagraph =
        selection.empty &&
        $from.depth === 1 &&
        $from.parent.type.name === 'paragraph' &&
        $from.parent.content.size === 0

      if (!isFocused || !isEmptyTopLevelParagraph) {
        closePlus()
        return
      }

      const coords = editor.view.coordsAtPos($from.pos)
      const canvasRect = canvas.getBoundingClientRect()
      const top = coords.top - canvasRect.top
      let left = dir === 'rtl'
        ? coords.right - canvasRect.left + 10
        : coords.left - canvasRect.left - 36

      // keep the control on-screen when the page padding is narrower than the offset (mobile)
      const minLeft = -canvasRect.left + 4
      const maxLeft = window.innerWidth - canvasRect.left - 30
      left = Math.max(minLeft, Math.min(left, maxLeft))

      plusControl.style.top = `${top}px`
      plusControl.style.left = `${left}px`
      plusControl.classList.add('is-visible')
    }

    plusControl.addEventListener('mousedown', () => {
      plusInteracting = true
    })
    document.addEventListener('mouseup', () => {
      plusInteracting = false
    })

    plusButton.addEventListener('click', (event) => {
      event.preventDefault()
      plusControl.classList.toggle('menu-open')
    })

    plusMenu.addEventListener('click', (event) => {
      const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-command]')
      if (!button) return
      event.preventDefault()
      closePlusMenu()
      runCommand(button.dataset.command || '')
    })

    document.addEventListener('click', (event) => {
      if (plusControl.contains(event.target as Node)) return
      closePlusMenu()
    })

    // ---------- floating selection toolbar ----------

    const bubbleActiveChecks: Record<string, () => boolean> = {
      bold: () => editor.isActive('bold'),
      italic: () => editor.isActive('italic'),
      heading: () => editor.isActive('heading', { level: 2 }),
      quote: () => editor.isActive('blockquote'),
      aside: () => editor.isActive('aside'),
      'bullet-list': () => editor.isActive('bulletList'),
      'ordered-list': () => editor.isActive('orderedList'),
      link: () => editor.isActive('link'),
      footnote: () => editor.isActive('footnote'),
      ltr: () => editor.isActive(directionBlockType(editor), { dir: 'ltr' }),
      rtl: () => editor.isActive(directionBlockType(editor), { dir: 'rtl' }),
    }

    const updateBubbleActiveState = () => {
      bubbleMenu.querySelectorAll<HTMLButtonElement>('[data-command]').forEach((button) => {
        const check = bubbleActiveChecks[button.dataset.command || '']
        button.classList.toggle('is-active', check ? check() : false)
      })
    }

    bubbleMenu.addEventListener('click', (event) => {
      const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-command]')
      if (!button) return
      event.preventDefault()
      runCommand(button.dataset.command || '')
    })

    imageMenu.addEventListener('click', (event) => {
      const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-command]')
      if (!button) return
      event.preventDefault()
      runCommand(button.dataset.command || '')
    })

    embedMenu.addEventListener('click', (event) => {
      const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-command]')
      if (!button) return
      event.preventDefault()
      runCommand(button.dataset.command || '')
    })

    // ---------- autosave ----------

    let saveTimer = 0
    let inFlight: AbortController | null = null

    const scheduleSave = () => {
      if (docLoadFailed) return
      setState('idle')
      window.clearTimeout(saveTimer)
      saveTimer = window.setTimeout(() => void save(), 700)
    }

    const save = async () => {
      syncDoc()
      inFlight?.abort()
      inFlight = new AbortController()
      setState('saving')

      const body = new URLSearchParams()
      body.set('title', titleInput.value)
      body.set('doc', docInput.value)

      try {
        const response = await fetch(saveURL, {
          method: 'POST',
          body,
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          signal: inFlight.signal,
        })
        if (!response.ok) throw new Error(`save failed: ${response.status}`)
        setState('saved')
      } catch (error) {
        if ((error as Error).name !== 'AbortError') {
          setState('error')
          throw error
        }
      }
    }

    const uploadImage = async (file: File) => {
      const body = new FormData()
      body.set('image', file)
      setState('saving')

      try {
        const response = await fetch('/api/uploads/images', {
          method: 'POST',
          body,
          credentials: 'same-origin',
        })
        if (!response.ok) throw new Error(`upload failed: ${response.status}`)
        const payload = (await response.json()) as { url?: string }
        if (!payload.url) throw new Error('upload response missing url')
        editor.chain().focus().setImage({ src: payload.url, alt: file.name }).run()
        focusParagraphAfterBlock(editor)
        syncDoc()
        await save()
      } catch {
        setState('error')
      }
    }

    titleInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') event.preventDefault()
    })

    titleInput.addEventListener('input', scheduleSave)

    imageInput.addEventListener('change', () => {
      const file = imageInput.files?.[0]
      imageInput.value = ''
      if (!file) return
      void uploadImage(file)
    })

    editor.view.dom.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') return
      const { selection } = editor.state
      if (!(selection instanceof NodeSelection) || !ATOM_BLOCK_TYPES.has(selection.node.type.name)) return
      event.preventDefault()
      focusParagraphAfterBlock(editor)
      syncDoc()
    })

    form.addEventListener('submit', (event) => {
      event.preventDefault()
      void save()
    })

    publishForm?.addEventListener('submit', (event) => {
      event.preventDefault()
      void save().then(() => publishForm.submit())
    })

    draftForm?.addEventListener('submit', (event) => {
      event.preventDefault()
      void save().then(() => draftForm.submit())
    })

    window.addEventListener('resize', () => updatePlus())

    window.addEventListener('pagehide', () => {
      if (docLoadFailed) return
      syncDoc()
      const body = new URLSearchParams()
      body.set('title', titleInput.value)
      body.set('doc', docInput.value)
      const blob = new Blob([body.toString()], { type: 'application/x-www-form-urlencoded' })
      navigator.sendBeacon(saveURL, blob)
    })

    if (docLoadFailed) {
      setState('error')
    } else {
      syncDoc()
      updateWordCount()
      updatePlus()
      updateBubbleActiveState()
      setState('saved')
    }
  }
}

function countDocWords(value: unknown): number {
  if (!value || typeof value !== 'object') return 0
  const doc = value as { content?: Array<{ text?: string; content?: unknown[] }> }
  let count = 0
  const walk = (nodes?: Array<{ text?: string; content?: unknown[] }>) => {
    if (!nodes) return
    for (const node of nodes) {
      if (typeof node.text === 'string' && node.text.trim()) {
        count += node.text.trim().split(/\s+/).filter(Boolean).length
      }
      walk(node.content as Array<{ text?: string; content?: unknown[] }> | undefined)
    }
  }
  walk(doc.content)
  return count
}

function setEmbed(editor: Editor, embedPrompt: string, embedInvalid: string) {
  const url = window.prompt(embedPrompt)
  if (url === null || url.trim() === '') return
  const attrs = parseEmbedURL(url)
  if (!attrs) {
    window.alert(embedInvalid)
    return
  }
  editor.chain().focus().insertContent({ type: 'embed', attrs }).run()
  focusParagraphAfterBlock(editor)
}

function setLink(editor: Editor, linkPrompt: string) {
  const current = editor.getAttributes('link').href as string | undefined
  const href = window.prompt(linkPrompt, current || 'https://')
  if (href === null) return
  if (href.trim() === '') {
    editor.chain().focus().unsetLink().run()
    return
  }
  editor.chain().focus().extendMarkRange('link').setLink({ href: href.trim() }).run()
}

// Direction is a block-level toggle (paragraph/heading, or the enclosing
// blockquote so a whole quote flips together) rather than a text-run mark:
// a quote's border/alignment need to flip along with its bidi text, not
// just the characters inside it.
function directionBlockType(editor: Editor): 'blockquote' | 'aside' | 'heading' | 'paragraph' {
  if (editor.isActive('blockquote')) return 'blockquote'
  if (editor.isActive('aside')) return 'aside'
  if (editor.isActive('heading')) return 'heading'
  return 'paragraph'
}

function toggleDirection(editor: Editor, dir: 'ltr' | 'rtl') {
  const type = directionBlockType(editor)
  if (editor.isActive(type, { dir })) {
    editor.chain().focus().updateAttributes(type, { dir: null }).run()
    return
  }
  editor.chain().focus().updateAttributes(type, { dir }).run()
}

function nextFootnoteId(editor: Editor): string {
  let max = 0
  editor.state.doc.descendants((node) => {
    if (!node.isText) return
    for (const mark of node.marks) {
      if (mark.type.name !== 'footnote') continue
      const id = String(mark.attrs.id || '')
      const match = /^fn(\d+)$/.exec(id)
      if (match) max = Math.max(max, Number(match[1]))
    }
  })
  return `fn${max + 1}`
}

function setFootnote(editor: Editor, footnotePrompt: string) {
  const current = editor.getAttributes('footnote') as { id?: string; text?: string }
  const text = window.prompt(footnotePrompt, current.text || '')
  if (text === null) return
  if (text.trim() === '') {
    editor.chain().focus().extendMarkRange('footnote').unsetMark('footnote').run()
    return
  }
  const id = current.id || nextFootnoteId(editor)
  editor
    .chain()
    .focus()
    .extendMarkRange('footnote')
    .setMark('footnote', { id, text: text.trim() })
    .run()
}
