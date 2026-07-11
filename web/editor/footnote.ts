import { Mark, mergeAttributes } from '@tiptap/core'

export const Footnote = Mark.create({
  name: 'footnote',
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
    }
  },
  parseHTML() {
    return [{ tag: 'sup.fn-ref' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['sup', mergeAttributes({ class: 'fn-ref' }, HTMLAttributes), 0]
  },
})
