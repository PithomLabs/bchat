import { Button, Input, Modal, ModalClose, ModalDialog, Option, Select, Sheet, Table, Typography, ButtonGroup, Chip, IconButton } from "@mui/joy";
import cx from "clsx";
import { ExternalLinkIcon, PlusIcon, TrashIcon, WorkflowIcon, LayoutListIcon, KanbanIcon, ListFilterIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { Link, useSearchParams } from "react-router-dom";
import useCurrentUser from "@/hooks/useCurrentUser";
import MobileHeader from "@/components/MobileHeader";
import { memoStore } from "@/store/v2";
import { Memo, Visibility, MemoRelation_Type } from "@/types/proto/api/v1/memo_service";
import TicketKanban from "@/components/TicketKanban";
import MemoView from "@/components/MemoView";
import MemoEditor from "@/components/MemoEditor";
import UserAvatar from "@/components/UserAvatar";
import { userStore } from "@/store/v2";

interface Ticket {
    id: number;
    title: string;
    description: string;
    status: string;
    priority: string;
    type?: string;
    creatorId: number;
    assigneeId?: number;
    createdTs: number;
    updatedTs: number;
    tags?: string[];
}

interface User {
    name: string;
    id: number;
    username: string;
    nickname?: string;
    avatarUrl?: string;
}

// Helper to extract memo UID from description URL like "/m/xyz"
function extractMemoUidFromDescription(description: string): string | null {
    if (!description) return null;
    const match = description.match(/^\/m\/(.+)$/);
    return match ? match[1] : null;
}

const Tickets = observer(() => {
    const currentUser = useCurrentUser();
    const [searchParams, setSearchParams] = useSearchParams();
    const [tickets, setTickets] = useState<Ticket[]>([]);
    const [users, setUsers] = useState<User[]>([]);
    const [showCreateDialog, setShowCreateDialog] = useState(false);
    const [editingTicket, setEditingTicket] = useState<Ticket | null>(null);
    const [viewMode, setViewMode] = useState<"list" | "board">("list");


    // Delete confirmation modal state
    const [deleteModalOpen, setDeleteModalOpen] = useState(false);
    const [ticketToDelete, setTicketToDelete] = useState<Ticket | null>(null);
    const [isDeleting, setIsDeleting] = useState(false);

    // Form state
    const [title, setTitle] = useState("");
    const [status, setStatus] = useState("OPEN");
    const [priority, setPriority] = useState("MEDIUM");
    const [type, setType] = useState("TASK");
    const [assigneeId, setAssigneeId] = useState<number | null>(null);
    const [description, setDescription] = useState("");

    // Comments state
    const [relatedMemos, setRelatedMemos] = useState<Memo[]>([]);
    const [showCommentEditor, setShowCommentEditor] = useState(false);
    const [isCreatingDescription, setIsCreatingDescription] = useState(false);

    useEffect(() => {
        fetchTickets();
        fetchUsers();
    }, [searchParams]);

    // Handle reactive refresh of comments when store changes
    useEffect(() => {
        if (showCreateDialog && editingTicket) {
            loadRelatedMemos(editingTicket, { skipCache: true });
        }
    }, [memoStore.state.stateId]);

    const fetchUsers = async () => {
        try {
            const response = await fetch("/api/v1/tickets/assignees");
            if (!response.ok) throw new Error("Failed to fetch assignees");
            const data = await response.json();
            setUsers(data || []);
        } catch (error) {
            console.error("Error loading assignees:", error);
        }
    };

    function getUserDisplayName(userId?: number): string {
        if (!userId) return "-";
        const user = users.find((u) => u.id === userId);
        if (!user) return `User ${userId}`;
        return user.nickname || user.username;
    }

    const fetchTickets = async () => {
        try {
            // Apply filters from URL if needed. For now basic fetch.
            const response = await fetch("/api/v1/tickets" + window.location.search);
            if (!response.ok) throw new Error("Failed to fetch tickets");
            const data = await response.json();
            setTickets(data);
        } catch (error) {
            toast.error("Error loading tickets");
        }
    };

    // Load related memos (comments) for a ticket
    const loadRelatedMemos = async (ticket: Ticket, options?: { skipCache?: boolean }) => {
        const memoUid = extractMemoUidFromDescription(ticket.description);
        if (!memoUid) {
            setRelatedMemos([]);
            return;
        }

        try {
            // 1. Get the main description memo
            const memoName = `memos/${memoUid}`;
            const memo = await memoStore.getOrFetchMemoByName(memoName, options);

            // 2. Fetch relations (comments)
            // Filter relations where type is COMMENT and it relates TO this memo
            const comments = memo.relations
                .filter(r => r.relatedMemo?.name === memo.name && r.type === MemoRelation_Type.COMMENT)
                .map(r => r.memo?.name)
                .filter(Boolean) as string[];

            if (comments.length > 0) {
                const memoObjects = await Promise.all(comments.map(name => memoStore.getOrFetchMemoByName(name)));
                setRelatedMemos(memoObjects);
            } else {
                setRelatedMemos([]);
            }
        } catch (error) {
            console.error("Failed to load related memos", error);
        }
    };

    const handleCreateOrUpdate = async () => {
        if (!title) {
            toast.error("Title is required");
            return;
        }

        // Validate that description is a valid memo link
        // We use the helper isMemoLink to check
        let checkingDesc = description;
        if (editingTicket) {
            checkingDesc = editingTicket.description;
        }
        if (!isMemoLink(checkingDesc)) {
            toast.error("A memo description is required. Please use 'Add description (Create Memo)' to link a memo.");
            return;
        }
        try {
            let memoUrl = description || "";
            if (editingTicket) {
                memoUrl = editingTicket.description;
            } else {
                // If creating a new ticket, user might have pasted a URL or left it empty
                memoUrl = description;
            }

            const payload = {
                title,
                description: memoUrl,
                status,
                priority,
                type,
                assigneeId: assigneeId || undefined
            };

            let response;
            if (editingTicket) {
                response = await fetch(`/api/v1/tickets/${editingTicket.id}`, {
                    method: "PATCH",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(payload),
                });
            } else {
                response = await fetch("/api/v1/tickets", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(payload),
                });
            }

            if (!response.ok) {
                const data = await response.json().catch(() => null);
                const message = data?.message || (await response.text()) || "Failed to save ticket";
                throw new Error(message);
            }

            toast.success(editingTicket ? "Ticket updated" : "Ticket created");
            setShowCreateDialog(false);
            setEditingTicket(null);
            resetForm();
            fetchTickets();
        } catch (error: any) {
            console.error(error);
            toast.error("Operation failed: " + (error.details || error.message));
        }
    };

    const handleCommentCreated = async (commentName: string) => {
        await memoStore.getOrFetchMemoByName(commentName);
        if (editingTicket) {
            loadRelatedMemos(editingTicket, { skipCache: true });
        }
        setShowCommentEditor(false);
    };

    const handleDescriptionCreated = async (memoName: string) => {
        const memoUid = memoName.split("/").pop();
        if (memoUid) {
            setDescription(`/m/${memoUid}`);
        }
        setIsCreatingDescription(false);
    };

    const openDeleteModal = (ticket: Ticket) => {
        setTicketToDelete(ticket);
        setDeleteModalOpen(true);
    };

    const confirmDelete = async () => {
        if (!ticketToDelete) return;
        setIsDeleting(true);
        try {
            const response = await fetch(`/api/v1/tickets/${ticketToDelete.id}`, {
                method: "DELETE",
                headers: { "Content-Type": "application/json" },
            });
            if (!response.ok) throw new Error("Failed to delete ticket");
            toast.success("Ticket deleted");
            setDeleteModalOpen(false);
            setTicketToDelete(null);
            fetchTickets();
        } catch (error: any) {
            toast.error("Failed to delete ticket: " + error.message);
        } finally {
            setIsDeleting(false);
        }
    };

    const resetForm = () => {
        setTitle("");
        setStatus("OPEN");
        setPriority("MEDIUM");
        setType("TASK");
        setAssigneeId(null);
        setDescription("");
        setRelatedMemos([]);
        setIsCreatingDescription(false);
    };

    const openEdit = async (ticket: Ticket) => {
        setEditingTicket(ticket);
        setTitle(ticket.title);
        setStatus(ticket.status);
        setPriority(ticket.priority);
        setType(ticket.type || "TASK");
        setAssigneeId(ticket.assigneeId || null);
        setDescription(ticket.description);
        setShowCreateDialog(true);
        loadRelatedMemos(ticket);
    };

    const openCreate = () => {
        setEditingTicket(null);
        resetForm();
        setShowCreateDialog(true);
    };

    const handleStatusChange = async (ticketId: number, newStatus: string) => {
        try {
            const response = await fetch(`/api/v1/tickets/${ticketId}`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ status: newStatus }),
            });
            if (!response.ok) throw new Error("Failed to update status");
            // Optimistic or refetch? Refetch is safer.
            fetchTickets();
        } catch (error) {
            toast.error("Failed to move ticket");
        }
    };

    function isMemoLink(description: string): boolean {
        return description?.startsWith("/m/");
    }

    return (
        <section className="@container w-full max-w-7xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
            <MobileHeader />
            <div className="w-full px-4 sm:px-6">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-6">
                    <div className="flex items-center gap-2">
                        <WorkflowIcon className="w-8 h-8 text-gray-500" />
                        <h1 className="text-2xl font-bold text-gray-800 dark:text-gray-200">Tickets</h1>
                    </div>
                    <div className="flex items-center gap-2">
                        <ButtonGroup>
                            <IconButton
                                variant={viewMode === "list" ? "solid" : "outlined"}
                                onClick={() => setViewMode("list")}
                            >
                                <LayoutListIcon />
                            </IconButton>
                            <IconButton
                                variant={viewMode === "board" ? "solid" : "outlined"}
                                onClick={() => setViewMode("board")}
                            >
                                <KanbanIcon />
                            </IconButton>
                        </ButtonGroup>
                        <Button startDecorator={<PlusIcon />} onClick={openCreate}>
                            New Ticket
                        </Button>
                    </div>
                </div>

                {viewMode === "list" ? (
                    <div className="w-full overflow-x-auto bg-white dark:bg-zinc-900 rounded-lg shadow-sm border border-gray-200 dark:border-zinc-800">
                        <Table hoverRow>
                            <thead>
                                <tr>
                                    <th style={{ width: "60px" }}>ID</th>
                                    <th>Type</th>
                                    <th>Title</th>
                                    <th>Description</th>
                                    <th>Status</th>
                                    <th>Priority</th>
                                    <th>Assignee</th>
                                    <th>Updated</th>
                                    <th style={{ width: "80px" }}></th>
                                </tr>
                            </thead>
                            <tbody>
                                {tickets.map((ticket) => (
                                    <tr key={ticket.id}>
                                        <td>#{ticket.id}</td>
                                        <td>
                                            <Chip size="sm" variant="outlined">{ticket.type || "TASK"}</Chip>
                                        </td>
                                        <td className="font-medium cursor-pointer text-blue-600 hover:underline" onClick={() => openEdit(ticket)}>
                                            {ticket.title}
                                        </td>
                                        <td className="text-sm font-mono text-gray-500 truncate max-w-[200px]">
                                            {isMemoLink(ticket.description) ? (
                                                <a href={ticket.description} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline flex items-center gap-1">
                                                    {ticket.description} <ExternalLinkIcon className="w-3 h-3" />
                                                </a>
                                            ) : (
                                                ticket.description
                                            )}
                                        </td>
                                        <td>
                                            <Chip
                                                size="sm" variant="soft"
                                                color={ticket.status === "OPEN" ? "success" : ticket.status === "IN_PROGRESS" ? "warning" : "neutral"}
                                            >
                                                {ticket.status}
                                            </Chip>
                                        </td>
                                        <td>
                                            <span className={cx(
                                                "text-xs font-semibold",
                                                ticket.priority === "HIGH" && "text-red-600",
                                                ticket.priority === "MEDIUM" && "text-yellow-600",
                                                ticket.priority === "LOW" && "text-blue-600"
                                            )}>
                                                {ticket.priority}
                                            </span>
                                        </td>
                                        <td>
                                            <span className="text-sm text-gray-600 dark:text-gray-400">
                                                {getUserDisplayName(ticket.assigneeId)}
                                            </span>
                                        </td>
                                        <td>{new Date(ticket.updatedTs * 1000).toLocaleDateString()}</td>
                                        <td>
                                            <IconButton size="sm" color="danger" onClick={() => openDeleteModal(ticket)}>
                                                <TrashIcon className="w-4 h-4" />
                                            </IconButton>
                                        </td>
                                    </tr>
                                ))}
                                {tickets.length === 0 && (
                                    <tr>
                                        <td colSpan={8} className="text-center py-8 text-gray-500">
                                            No tickets found.
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </Table>
                    </div>
                ) : (
                    <TicketKanban
                        tickets={tickets}
                        users={users}
                        onTicketClick={openEdit}
                        onStatusChange={handleStatusChange}
                    />
                )}
            </div>

            {showCreateDialog && (
                <div className="fixed inset-0 z-50 flex items-center justify-end bg-black/50">
                    <Sheet
                        className="w-full max-w-2xl h-full p-6 shadow-2xl overflow-y-auto"
                        variant="outlined"
                        sx={{ borderLeft: "1px solid", borderColor: "divider" }}
                    >
                        <div className="flex justify-between items-center mb-6">
                            <h2 className="text-xl font-bold">{editingTicket ? `Edit Ticket #${editingTicket.id}` : "New Ticket"}</h2>
                            <IconButton onClick={() => setShowCreateDialog(false)}><ModalClose /></IconButton>
                        </div>

                        <div className="flex flex-col gap-4">
                            <div>
                                <label className="block text-sm font-medium mb-1">Title</label>
                                <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Ticket title" />
                            </div>

                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-sm font-medium mb-1">Type</label>
                                    <Select value={type} onChange={(_, val) => setType(val || "TASK")}>
                                        <Option value="TASK">Task</Option>
                                        <Option value="BUG">Bug</Option>
                                        <Option value="STORY">Story</Option>
                                    </Select>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium mb-1">Status</label>
                                    <Select value={status} onChange={(_, val) => setStatus(val || "OPEN")}>
                                        <Option value="OPEN">Open</Option>
                                        <Option value="IN_PROGRESS">In Progress</Option>
                                        <Option value="CLOSED">Closed</Option>
                                    </Select>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium mb-1">Priority</label>
                                    <Select value={priority} onChange={(_, val) => setPriority(val || "MEDIUM")}>
                                        <Option value="LOW">Low</Option>
                                        <Option value="MEDIUM">Medium</Option>
                                        <Option value="HIGH">High</Option>
                                    </Select>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium mb-1">Assignee</label>
                                    <Select
                                        value={assigneeId}
                                        onChange={(_, val) => setAssigneeId(val)}
                                        placeholder="Select assignee"
                                    >
                                        <Option value={null as unknown as number}>Unassigned</Option>
                                        {users.map((user) => (
                                            <Option key={user.id} value={user.id}>
                                                {user.username}
                                            </Option>
                                        ))}
                                    </Select>
                                </div>
                            </div>

                            <div>
                                <label className="block text-sm font-medium mb-1">Memo URL (Description)</label>
                                {editingTicket && isMemoLink(editingTicket.description) ? (
                                    <div className="p-2 border rounded-md bg-gray-50 dark:bg-zinc-900 flex items-center gap-2">
                                        <a href={editingTicket.description} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline flex items-center gap-1">
                                            {editingTicket.description} <ExternalLinkIcon className="w-4 h-4" />
                                        </a>
                                    </div>
                                ) : (
                                    <div className="flex flex-col gap-2">
                                        {!editingTicket && !description && !isCreatingDescription ? (
                                            <Button
                                                variant="outlined"
                                                color="neutral"
                                                startDecorator={<PlusIcon className="w-4 h-4" />}
                                                onClick={() => setIsCreatingDescription(true)}
                                            >
                                                Add description (Create Memo)
                                            </Button>
                                        ) : isCreatingDescription ? (
                                            <div className="border rounded-lg p-3 bg-white dark:bg-zinc-800">
                                                <MemoEditor
                                                    cacheKey="ticket-description-new"
                                                    placeholder="Write ticket description..."
                                                    autoFocus
                                                    defaultVisibility={Visibility.PUBLIC}
                                                    onConfirm={handleDescriptionCreated}
                                                    onCancel={() => setIsCreatingDescription(false)}
                                                />
                                            </div>
                                        ) : (
                                            <div className="flex flex-col gap-2">
                                                <Input
                                                    value={editingTicket ? editingTicket.description : description}
                                                    onChange={(e) => !editingTicket && setDescription(e.target.value)}
                                                    placeholder="Paste Memo URL or leave empty"
                                                    readOnly={!!editingTicket}
                                                    variant="outlined"
                                                />
                                                {!editingTicket && description && isMemoLink(description) && (
                                                    <div className="text-xs text-green-600 flex items-center gap-1 ml-1">
                                                        Memo linked successfully!
                                                        <Button size="sm" variant="plain" color="danger" onClick={() => setDescription("")}>Clear</Button>
                                                    </div>
                                                )}
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            <div className="flex justify-end gap-2 pt-4 border-b pb-4 mb-4">
                                <Button variant="outlined" color="neutral" onClick={() => setShowCreateDialog(false)}>
                                    Cancel
                                </Button>
                                <Button onClick={handleCreateOrUpdate}>
                                    {editingTicket ? "Update Ticket" : "Create Ticket"}
                                </Button>
                            </div>

                            {/* Comments Section (only for existing tickets linked to a memo) */}
                            {editingTicket && isMemoLink(editingTicket.description) && (
                                <div className="mt-4">
                                    <Typography level="title-md" mb={2}>Comments ({relatedMemos.length})</Typography>

                                    <div className="flex flex-col gap-3 mb-4">
                                        {relatedMemos.map(memo => (
                                            <CommentItem key={memo.name} memo={memo} />
                                        ))}
                                    </div>

                                    {!showCommentEditor ? (
                                        <Button variant="soft" color="neutral" onClick={() => setShowCommentEditor(true)}>
                                            Add Comment
                                        </Button>
                                    ) : (
                                        <div className="border rounded-lg p-3 bg-white dark:bg-zinc-800">
                                            <MemoEditor
                                                cacheKey={`ticket-comment-${editingTicket.id}`}
                                                placeholder="Write a comment..."
                                                parentMemoName={`memos/${extractMemoUidFromDescription(editingTicket.description)}`}
                                                autoFocus
                                                defaultVisibility={Visibility.PUBLIC}
                                                onConfirm={handleCommentCreated}
                                                onCancel={() => setShowCommentEditor(false)}
                                            />
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    </Sheet>
                </div>
            )}

            <Modal open={deleteModalOpen} onClose={() => setDeleteModalOpen(false)}>
                <ModalDialog variant="outlined" role="alertdialog">
                    <ModalClose />
                    <Typography level="h4" startDecorator={<TrashIcon className="w-5 h-5 text-red-500" />}>
                        Delete Ticket
                    </Typography>
                    <div className="flex justify-end gap-2 mt-4">
                        <Button variant="outlined" color="neutral" onClick={() => setDeleteModalOpen(false)}>Cancel</Button>
                        <Button color="danger" onClick={confirmDelete} loading={isDeleting}>Delete</Button>
                    </div>
                </ModalDialog>
            </Modal>
        </section>
    );
});

const CommentItem = observer(({ memo }: { memo: Memo }) => {
    const liveMemo = memoStore.getMemoByName(memo.name) || memo;
    const [creator, setCreator] = useState<User | undefined>(undefined);

    useEffect(() => {
        userStore.getOrFetchUserByName(liveMemo.creator).then((user) => {
            setCreator(user as any);
        });
    }, [liveMemo.creator]);

    // If the memo is explicitly deleted from store, don't show it
    if (!memoStore.getMemoByName(memo.name)) return null;

    return (
        <div className="border rounded-lg p-3 bg-gray-50 dark:bg-zinc-900/50">
            <div className="flex justify-between items-center mb-1">
                <div className="flex items-center gap-2">
                    {creator && <UserAvatar avatarUrl={creator.avatarUrl as any} className="w-5 h-5" />}
                    <span className="font-semibold text-sm">{creator ? (creator.nickname || creator.username) : liveMemo.creator}</span>
                </div>
                <span className="text-xs text-gray-500">{liveMemo.createTime ? new Date(liveMemo.createTime).toLocaleString() : ""}</span>
            </div>
            <MemoView memo={liveMemo} compact showCreator={false} />
        </div>
    );
});

export default Tickets;
