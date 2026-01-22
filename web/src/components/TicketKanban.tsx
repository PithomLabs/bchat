import { DndContext, DragEndEvent, DragOverlay, DragStartEvent, PointerSensor, useSensor, useSensors, closestCorners } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Avatar, Card, Chip, Typography } from "@mui/joy";
import { CheckCircleIcon, CircleIcon, ClockIcon } from "lucide-react";
import React, { useMemo, useState } from "react";
import cx from "clsx";

// Types mirrored from Tickets.tsx (will be shared later ideally, but kept here for isolation)
export interface Ticket {
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
}

interface User {
  id: number;
  username: string;
  nickname?: string;
}

interface Props {
  tickets: Ticket[];
  users: User[];
  onTicketClick: (ticket: Ticket) => void;
  onStatusChange: (ticketId: number, newStatus: string) => void;
}

const COLUMNS = [
  { id: "OPEN", title: "Open", icon: CircleIcon, color: "text-green-600" },
  { id: "IN_PROGRESS", title: "In Progress", icon: ClockIcon, color: "text-yellow-600" },
  { id: "CLOSED", title: "Closed", icon: CheckCircleIcon, color: "text-gray-600" },
];

const TicketCard = ({
  ticket,
  users,
  onClick,
  isOverlay = false,
}: {
  ticket: Ticket;
  users: User[];
  onClick?: () => void;
  isOverlay?: boolean;
}) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: ticket.id,
    data: { ticket },
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.3 : 1,
  };

  const assignee = users.find((u) => u.id === ticket.assigneeId);

  return (
    <Card
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      size="sm"
      className={cx(
        "cursor-pointer hover:shadow-md transition-shadow",
        isOverlay ? "shadow-xl rotate-2 cursor-grabbing" : ""
      )}
      onClick={onClick}
    >
      <div className="flex justify-between items-start gap-2">
        <Typography level="title-sm" className="line-clamp-2 mb-1">
          {ticket.title}
        </Typography>
        {ticket.type && (
          <Chip size="sm" variant="outlined" color="primary">
            {ticket.type}
          </Chip>
        )}
      </div>
      <div className="flex justify-between items-center mt-2">
        <Chip
          size="sm"
          variant="soft"
          color={
            ticket.priority === "HIGH" ? "danger" : ticket.priority === "MEDIUM" ? "warning" : "primary"
          }
        >
          {ticket.priority}
        </Chip>
        {assignee && (
          <Avatar size="sm" variant="soft" color="neutral">
            {(assignee.nickname || assignee.username)[0].toUpperCase()}
          </Avatar>
        )}
      </div>
    </Card>
  );
};

const TicketKanban = ({ tickets, users, onTicketClick, onStatusChange }: Props) => {
  const [activeId, setActiveId] = useState<number | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 5,
      },
    })
  );

  const columns = useMemo(() => {
    const cols: Record<string, Ticket[]> = {
      OPEN: [],
      IN_PROGRESS: [],
      CLOSED: [],
    };
    tickets.forEach((t) => {
      const status = t.status || "OPEN";
      if (cols[status]) {
        cols[status].push(t);
      } else {
        // Fallback for unknown status
        cols["OPEN"].push(t);
      }
    });
    return cols;
  }, [tickets]);

  const handleDragStart = (event: DragStartEvent) => {
    setActiveId(event.active.id as number);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    setActiveId(null);

    if (!over) return;

    const activeId = active.id as number;
    const overId = over.id as string | number;

    // Find status of dropped container
    let newStatus = "";

    // Check if dropped on a column (droppable id = status)
    if (COLUMNS.some(c => c.id === overId)) {
      newStatus = overId as string;
    } else {
      // Dropped on another card?
      // For simplicity in this basic version, we just check if it's over a column container or a card within it.
      // But @dnd-kit "over" could be a card.
      // We can find the container of the over item.
      const overTicket = tickets.find(t => t.id === overId);
      if (overTicket) {
        newStatus = overTicket.status;
      }
    }

    const activeTicket = tickets.find((t) => t.id === activeId);
    if (activeTicket && newStatus && activeTicket.status !== newStatus) {
      onStatusChange(activeId, newStatus);
    }
  };

  const activeTicket = activeId ? tickets.find((t) => t.id === activeId) : null;

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <div className="flex flex-row gap-4 h-full overflow-x-auto pb-4 w-full">
        {COLUMNS.map((col) => (
          <div
            key={col.id}
            className="flex-1 min-w-[280px] bg-gray-50 dark:bg-zinc-900 rounded-lg p-2 border border-gray-200 dark:border-zinc-800 flex flex-col"
          >
            <div className="flex items-center gap-2 mb-3 px-2 pt-1">
              <col.icon className={cx("w-4 h-4", col.color)} />
              <Typography level="title-sm" fontWeight="bold">
                {col.title}
              </Typography>
              <Chip size="sm" variant="outlined">
                {columns[col.id]?.length || 0}
              </Chip>
            </div>

            <SortableContext
              id={col.id}
              items={columns[col.id]?.map((t) => t.id) || []}
              strategy={verticalListSortingStrategy}
            >
              <div className="flex flex-col gap-2 flex-grow min-h-[100px]">
                {columns[col.id]?.map((ticket) => (
                  <TicketCard
                    key={ticket.id}
                    ticket={ticket}
                    users={users}
                    onClick={() => onTicketClick(ticket)}
                  />
                ))}
                {/* Droppable area filler if empty */}
                {columns[col.id]?.length === 0 && (
                  <div className="h-full w-full opacity-0" id={col.id}>Drop here</div>
                )}
              </div>
            </SortableContext>
          </div>
        ))}
      </div>

      <DragOverlay>
        {activeTicket ? (
          <TicketCard ticket={activeTicket} users={users} isOverlay />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
};

export default TicketKanban;
