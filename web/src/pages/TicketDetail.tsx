import { Button, Input, Textarea } from "@mui/joy";
import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslate } from "@/utils/i18n";
import axios from "axios";
import { toast } from "react-hot-toast";

interface Ticket {
    id: number;
    title: string;
    description: string;
    status: string;
    priority: string;
    creatorId: number;
    assigneeId?: number;
    createdTs: number;
    updatedTs: number;
    type: string;
    tags: string[];
}

const TicketDetail = () => {
    const { id } = useParams();
    const navigate = useNavigate();
    const t = useTranslate();
    const [ticket, setTicket] = useState<Ticket | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchTicket = async () => {
            try {
                const { data } = await axios.get<Ticket>(`/api/v1/tickets/${id}`);
                setTicket(data);
            } catch (error: any) {
                console.error("Failed to fetch ticket", error);
                toast.error(`Failed to fetch ticket: ${error.response?.data?.message || error.message}`);
                navigate("/tickets"); // Redirect back to list on error
            } finally {
                setLoading(false);
            }
        };

        if (id) {
            fetchTicket();
        }
    }, [id, navigate]);

    if (loading) {
        return <div className="w-full h-full flex justify-center items-center">Loading...</div>;
    }

    if (!ticket) {
        return <div>Ticket not found</div>;
    }

    return (
        <section className="w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
            <div className="w-full px-4 sm:px-6">
                <div className="w-full shadow flex flex-col justify-start items-start px-4 py-3 rounded-xl bg-white dark:bg-zinc-800 text-black dark:text-gray-300">
                    <div className="flex justify-between w-full mb-4">
                        <h1 className="text-2xl font-bold">Ticket #{ticket.id}: {ticket.title}</h1>
                        <Button onClick={() => navigate("/tickets")}>Back to List</Button>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 w-full">
                        <div>
                            <p className="text-sm text-gray-500">Status</p>
                            <p className="font-semibold">{ticket.status}</p>
                        </div>
                        <div>
                            <p className="text-sm text-gray-500">Priority</p>
                            <p className="font-semibold">{ticket.priority}</p>
                        </div>
                        <div>
                            <p className="text-sm text-gray-500">Type</p>
                            <p className="font-semibold">{ticket.type}</p>
                        </div>
                        <div>
                            <p className="text-sm text-gray-500">Assignee</p>
                            <p className="font-semibold">{ticket.assigneeId || "Unassigned"}</p>
                        </div>
                    </div>

                    <div className="mt-6 w-full">
                        <p className="text-sm text-gray-500 mb-2">Description</p>
                        <div className="p-4 border rounded-md whitespace-pre-wrap dark:border-gray-700">
                            {ticket.description}
                        </div>
                    </div>
                </div>
            </div>
        </section>
    );
};

export default TicketDetail;
