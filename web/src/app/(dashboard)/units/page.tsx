import { redirect } from "next/navigation";

// The units list now lives in the object explorer (browse + filter + drawer). Create/detail routes
// (units/new, units/[unitId]) are unchanged.
export default function UnitsPage() {
  redirect("/explore/unit");
}
