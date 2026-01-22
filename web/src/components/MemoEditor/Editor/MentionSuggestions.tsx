import Fuse from "fuse.js";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import getCaretCoordinates from "textarea-caret";
import OverflowTip from "@/components/kit/OverflowTip";
import { userStore } from "@/store/v2";
import { cn } from "@/utils";
import { EditorRefActions } from ".";
import { User } from "@/types/proto/api/v1/user_service";

type Props = {
    editorRef: React.RefObject<HTMLTextAreaElement>;
    editorActions: React.ForwardedRef<EditorRefActions>;
};

type Position = { left: number; top: number; height: number };

const MentionSuggestions = observer(({ editorRef, editorActions }: Props) => {
    const [position, setPosition] = useState<Position | null>(null);
    const [selected, select] = useState(0);
    const selectedRef = useRef(selected);
    selectedRef.current = selected;

    useEffect(() => {
        userStore.fetchUsers();
    }, []);

    const users = Object.values(userStore.state.userMapByName);

    const hide = () => setPosition(null);

    const getCurrentWord = (): [word: string, startIndex: number] => {
        const editor = editorRef.current;
        if (!editor) return ["", 0];
        const cursorPos = editor.selectionEnd;
        const before = editor.value.slice(0, cursorPos).match(/\S*$/) || { 0: "", index: cursorPos };
        const after = editor.value.slice(cursorPos).match(/^\S*/) || { 0: "" };
        return [before[0] + after[0], before.index ?? cursorPos];
    };

    const suggestionsRef = useRef<User[]>([]);
    suggestionsRef.current = (() => {
        const [word] = getCurrentWord();
        if (!word.startsWith("@")) return [];
        const search = word.slice(1).toLowerCase();
        if (!search) return users.slice(0, 20); // Show recent or all if empty? limit to 20

        const fuse = new Fuse(users, {
            keys: ["nickname", "username"],
            threshold: 0.3,
        });
        return fuse.search(search).map((result) => result.item).slice(0, 10);
    })();

    const isVisibleRef = useRef(false);
    isVisibleRef.current = !!(position && suggestionsRef.current.length > 0);

    const autocomplete = (user: User) => {
        if (!editorActions || !("current" in editorActions) || !editorActions.current) return;
        const [word, index] = getCurrentWord();
        editorActions.current.removeText(index, word.length);
        editorActions.current.insertText(`@${user.nickname || user.username} `);
        hide();
    };

    const handleKeyDown = (e: KeyboardEvent) => {
        if (!isVisibleRef.current) return;
        const suggestions = suggestionsRef.current;
        const selected = selectedRef.current;
        if (["Escape", "ArrowLeft", "ArrowRight"].includes(e.code)) hide();
        if ("ArrowDown" === e.code) {
            select((selected + 1) % suggestions.length);
            e.preventDefault();
            e.stopPropagation();
        }
        if ("ArrowUp" === e.code) {
            select((selected - 1 + suggestions.length) % suggestions.length);
            e.preventDefault();
            e.stopPropagation();
        }
        if (["Enter", "Tab"].includes(e.code)) {
            if (suggestions[selected]) {
                autocomplete(suggestions[selected]);
                e.preventDefault();
                e.stopPropagation();
            }
        }
    };

    const handleInput = () => {
        const editor = editorRef.current;
        if (!editor) return;

        select(0);
        const [word, index] = getCurrentWord();
        const currentChar = editor.value[editor.selectionEnd - 1]; // check char before cursor
        // The previous logic checked editor.value[editor.selectionEnd] which is char AFTER cursor usually?
        // Wait, TagSuggestions check `currentChar = editor.value[editor.selectionEnd]`.
        // If I type "@", cursor is after @. editor.selectionEnd points to next char.
        // If I type "@a", cursor is after a.
        // TagSuggestions logic: `const currentChar = editor.value[editor.selectionEnd];` -> This seems wrong if looking for active typing. 
        // It might be checking if we are INSIDE a tag?
        // Let's stick to simple "starts with @" logic for the word found around cursor.

        const isActive = word.startsWith("@");

        if (isActive) {
            const caretCordinates = getCaretCoordinates(editor, index);
            caretCordinates.top -= editor.scrollTop;
            setPosition(caretCordinates);
        } else {
            hide();
        }
    };

    const listenersAreRegisteredRef = useRef(false);
    const registerListeners = () => {
        const editor = editorRef.current;
        if (!editor || listenersAreRegisteredRef.current) return;
        editor.addEventListener("click", hide);
        editor.addEventListener("blur", hide);
        editor.addEventListener("keydown", handleKeyDown);
        editor.addEventListener("input", handleInput);
        listenersAreRegisteredRef.current = true;
    };
    useEffect(registerListeners, [!!editorRef.current]);

    if (!isVisibleRef.current || !position) return null;

    return (
        <div
            className="z-20 p-1 mt-1 -ml-2 absolute max-w-[14rem] gap-px rounded font-mono flex flex-col justify-start items-start overflow-auto shadow bg-zinc-100 dark:bg-zinc-700 max-h-48"
            style={{ left: position.left, top: position.top + position.height }}
        >
            {suggestionsRef.current.map((user, i) => (
                <div
                    key={user.name}
                    onMouseDown={(e) => {
                        e.preventDefault(); // Prevent blur
                        autocomplete(user);
                    }}
                    className={cn(
                        "rounded p-1 px-2 w-full truncate text-sm dark:text-gray-300 cursor-pointer hover:bg-zinc-200 dark:hover:bg-zinc-800",
                        i === selected ? "bg-zinc-300 dark:bg-zinc-600" : "",
                    )}
                >
                    <OverflowTip>{user.nickname || user.username} (@{user.username})</OverflowTip>
                </div>
            ))}
        </div>
    );
});

export default MentionSuggestions;
