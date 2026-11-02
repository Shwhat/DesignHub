// Inline comments: the text a user selected to comment on, and the
// comment to show when a highlight is clicked.
export interface Anchor {
  quote: string;
  prefix: string;
  suffix: string;
  field?: string;
}

export const inline = $state({
  pending: null as Anchor | null,
  focus: '',
  text: '', // plain text of the record shown, to tell whether quoted text still exists
  version: 0, // raised when comments change, so highlights redraw
});
