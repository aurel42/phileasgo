import { Subject } from "@microsoft/msfs-sdk";

export interface LogEntry {
    timestamp: number;
    level: "info" | "warn" | "error";
    message: string;
}

class LoggerService {
    public readonly logs = Subject.create<LogEntry[]>([]);
    private logHistory: LogEntry[] = [];
    private maxLogs = 50;

    public enabled = true;

    constructor() {
        // Global hook removed to prevent crashes/recursion.
        // Use Logger.info/warn/error explicitly.
    }

    public info(...args: any[]) {
        this.addLog("info", args);
        if (this.enabled) console.log(...args);
    }

    public warn(...args: any[]) {
        this.addLog("warn", args);
        if (this.enabled) console.warn(...args);
    }

    public error(...args: any[]) {
        this.addLog("error", args);
        if (this.enabled) console.error(...args);
    }

    private addLog(level: "info" | "warn" | "error", args: any[]) {
        const message = args.map(arg =>
            typeof arg === 'object' ? JSON.stringify(arg) : String(arg)
        ).join(" ");

        const entry: LogEntry = {
            timestamp: Date.now(),
            level,
            message
        };

        this.logHistory.unshift(entry);
        if (this.logHistory.length > this.maxLogs) {
            this.logHistory.pop();
        }

        this.logs.set([...this.logHistory]);
    }
}

export const Logger = new LoggerService();
