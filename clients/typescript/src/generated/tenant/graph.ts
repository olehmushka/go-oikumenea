/** A named hierarchy over the units (D-Graphs). Each graph is independently a DAG. */
export interface IGraph {
    /** The graph's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (e.g. command, operational); referenced by edges/closure/assignments. */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** The graph a subtree grant uses when none is named; exactly one default exists (seeded command). */
    'isDefault': boolean;
    /** Whether the PDP cascades subtree grants over this graph (D-DirectoryGraphs). command is locked TRUE. */
    'isAuthorityBearing': boolean;
}
