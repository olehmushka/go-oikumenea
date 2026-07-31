import { redirect } from "next/navigation";

// The audit log lives in the generic explorer since M58 ticket 1: /explore/audit serves the list
// AND the dashboard over the full nine filters, where this page fetched one page of 50 with two of
// them hard-coded and dropped the next-page token. Kept as a redirect because the path is in muscle
// memory and in bookmarks — the same treatment /persons and /units already have.
export default function AuditPage() {
  redirect("/explore/audit");
}
