
import { useCallback, useEffect, useRef, useState } from 'react';
import './App.css'

import { Tree } from 'react-arborist';
import type { NodeRendererProps } from 'react-arborist';
import type { NodeData } from './types';
import { buildFileTree } from './buildTree';


const padding = 20

function App() {
  const [entryPointPath, setEntryPointPath] = useState("")
  const [tree, setTree] = useState<NodeData[]>([])
  const [explainedPath, setExplainedPath] = useState<string[]>([])

  const resolveFiles = async (entryPointPathOverride?: string) => {
    setExplainedPath([])
    const req = await fetch(`/getEntryPointFiles?filePath=${encodeURI(entryPointPathOverride ?? entryPointPath)}`, {
      method: "GET",

    })

    const data = (await req.json()) as string[];

    const tree = buildFileTree(data)

    setTree(tree)

    console.log(tree)
  }

  const explainDependency = useCallback(async (filePath: string) => {
    const req = await fetch(`/explainDependency?filePath=${encodeURI(filePath)}&entryPoint=${encodeURI(entryPointPath)}`, {
      method: "GET",
    })

    const data = (await req.json()) as string[];

    setExplainedPath(data)
  }, [entryPointPath])

  const clear = () => {
    setEntryPointPath("")
    resolveFiles("")
  }

  useEffect(() => {
    resolveFiles()
  }, [])

  const Node = useCallback(function Node({ node, style, dragHandle }: NodeRendererProps<NodeData>) {
    return (
      <div style={style} ref={dragHandle} >
        <span style={{ marginRight: "4px" }} onClick={() => node.toggle()}>{node.isLeaf ? "📜" : "📁"}</span>
        <span onClick={() => node.isLeaf ? setEntryPointPath(node.data.id) : node.toggle()} style={{ fontFamily: "monospace" }}>
          {node.data.name}{node.isLeaf ? <span style={{ paddingLeft: 4 }} onClick={(e) => { e.stopPropagation(); explainDependency(node.id) }}>❔</span> : ` (${node.data.count})`}
        </span>
      </div>
    );
  }, [setEntryPointPath, explainDependency])

  const headerRef = useRef<HTMLDivElement>(null)

  const treeHeight = window.innerHeight - 2 * padding - (headerRef?.current?.getBoundingClientRect().height ?? 0)

  return (
    <div style={{ width: '100vw', height: '100vw', display: 'flex', flexDirection: "column", padding }}>
      <div style={{ display: 'flex', flexDirection: 'row', gap: 20, justifyContent: "center" }} ref={headerRef}>

        <input type="text" value={entryPointPath} onChange={(e) => setEntryPointPath(e.target.value)} style={{ width: 500 }} />
        <button onClick={() => resolveFiles()}>Resolve files</button>
        <button onClick={clear}>❌</button>

      </div>

      <Tree data={tree} indent={20} idAccessor={"id"} width={"100vw"} height={treeHeight}>
        {Node}
      </Tree>


      {explainedPath.length > 0 ? (
        <div style={{ position: "fixed", height: "100vh", width: "100vw", display: "flex", justifyContent: "center", alignItems: "center" }}>
          <div style={{ border: '1px solid white', borderRadius: 10, backgroundColor: '#242424', padding: 10, position: "relative" }}>
            <div style={{ position: 'absolute', top: 6, right: 6, cursor: 'pointer' }} onClick={() => setExplainedPath([])}>❌</div>
            <div style={{ display: 'flex', flexDirection: "column" }}>
              {explainedPath.map((filePath, idx) => (
                <span style={{ paddingLeft: idx * 6, fontFamily: 'monospace' }}>➞ {filePath}</span>
              ))}
            </div>
          </div>
        </div>) : null}
    </div>
  )
}

export default App
