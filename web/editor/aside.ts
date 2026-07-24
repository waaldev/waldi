import { Node, mergeAttributes } from '@tiptap/core'

// A quiet callout box for tangential remarks - same shape as Blockquote
// (wrap/lift a paragraph) but its own node so it can carry the paper-raised
// box styling instead of the blockquote's inset rule.
export const Aside = Node.create({
  name: 'aside',
  content: 'block+',
  group: 'block',
  defining: true,

  parseHTML() {
    return [{ tag: 'div.aside' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes({ class: 'aside' }, HTMLAttributes), 0]
  },

  addCommands() {
    return {
      toggleAside: () => ({ commands }) => commands.toggleWrap(this.name),
    }
  },
})

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    aside: {
      toggleAside: () => ReturnType
    }
  }
}
