import type { NodeData } from "./types";

/**
 * Build a tree of NodeData from file paths.
 * By default only includes files with extensions: ts|js|tsx|jsx|mjs|mjsx (case-insensitive).
 *
 * @param paths array of file paths (posix-style or with backslashes)
 * @param fileExtRegex optional regex to filter files (pass null/undefined to disable filtering)
 */
export function buildFileTree(
  paths: string[],
  fileExtRegex: RegExp = /\.(?:ts|js|tsx|jsx|mjs|mjsx|cjs)$/i
): NodeData[] {
  type NodeInternal = {
    name: string;
    id: string;
    children: Map<string, NodeInternal> | null; // null for files (leaf)
  };

  const root = new Map<string, NodeInternal>();

  for (const raw of paths) {
    const normalized = raw
      .replace(/\\/g, "/")
      .replace(/\/+/g, "/")
      .replace(/^\/|\/$/g, ""); // normalize slashes
    if (fileExtRegex && !fileExtRegex.test(normalized)) continue;

    const parts = normalized.split("/").filter(Boolean);
    if (parts.length === 0) continue;

    let currentMap = root;
    let parentPath = "";

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      parentPath = parentPath ? `${parentPath}/${part}` : part;
      const isLeaf = i === parts.length - 1;

      let node = currentMap.get(part);
      if (!node) {
        node = {
          name: part,
          id: parentPath,
          children: isLeaf ? null : new Map<string, NodeInternal>(),
        };
        currentMap.set(part, node);
      } else if (!isLeaf && node.children === null) {
        node.children = new Map<string, NodeInternal>();
      }

      if (!isLeaf) {
        currentMap = node.children!;
      }
    }
  }

  // Recursive conversion with count aggregation
  function convert(map: Map<string, NodeInternal>): NodeData[] {
    const out: NodeData[] = [];
    for (const node of map.values()) {
      if (node.children) {
        const children = convert(node.children);
        const count = children.reduce((sum, c) => sum + c.count, 0);
        out.push({
          id: node.id,
          name: node.name,
          count,
          children,
        });
      } else {
        out.push({
          id: node.id,
          name: node.name,
          count: 1, // file
        });
      }
    }
    return out;
  }
  const converted = convert(root);

  // Condense the tree by merging single-child nodes
  function condenseTree(nodes: NodeData[]): NodeData[] {
    return nodes.map(node => {
      if (!node.children) {
        // Leaf node, return as-is
        return node;
      }

      // Recursively condense children first
      const condensedChildren = condenseTree(node.children);
      
      // If this node has only one child and that child is not a leaf,
      // merge this node with its child
      if (condensedChildren.length === 1 && condensedChildren[0].children) {
        const child = condensedChildren[0];
        return {
          id: child.id, // Use child's full path as ID
          name: `${node.name}/${child.name}`, // Combine names with separator
          children: child.children,
          count: child.count
        };
      }

      // Otherwise, return node with condensed children
      return {
        ...node,
        children: condensedChildren
      };
    });
  }

  const condensed = condenseTree(converted);

  return [{
    id: 'root',
    name: 'root',
    children: condensed,
    count: condensed.reduce((sum, node) => sum+node.count, 0)
  }] as NodeData[]
}