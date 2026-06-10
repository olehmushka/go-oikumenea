import { redirect } from "next/navigation";

// The persons list now lives in the object explorer (browse + filter + drawer + bulk actions).
// Create/detail routes (persons/new, persons/[personId]) are unchanged.
export default function PersonsPage() {
  redirect("/explore/person");
}
