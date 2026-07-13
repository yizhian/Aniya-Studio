import { useContext } from 'react';
import {
  EditorContext,
  EditorRuntimeContext,
  type EditorRuntimeValue,
} from '../context/EditorContext';

export function useEditor() {
  return useContext(EditorContext);
}

export function useEditorRuntime(): EditorRuntimeValue {
  return useContext(EditorRuntimeContext);
}
