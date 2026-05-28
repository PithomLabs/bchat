#!/usr/bin/env python3
import sqlite3
import re
import os
import sys
import argparse
import hashlib

def get_hash(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()

def is_boilerplate_block(file_path, body):
    file_path_lower = file_path.lower()
    
    # 1. Reject by file extension or path
    if file_path_lower.endswith(('.js', '.css')):
        return True
    if any(kw in file_path_lower for kw in ['3hsp/', 'googletagmanager', 'google_tag_manager', 'google-analytics']):
        return True

    # 2. Reject by content analysis (minified script/stylesheet signature)
    lines = body.split('\n')
    for line in lines:
        line = line.strip()
        if len(line) > 500:
            spaces = line.count(' ')
            space_ratio = spaces / len(line)
            
            if space_ratio < 0.05:
                # Check for minified JS keywords/signatures
                js_signatures = ["(function(", "eval(", "window.", "document.", "var ", "const ", "let ", "function(", "dataLayer.push("]
                for sig in js_signatures:
                    if sig in line:
                        return True
                
                # Check for minified CSS signatures
                if '{' in line and '}' in line and ';' in line:
                    return True

    return False

def clean_source_content(content):
    # 1. Remove HTML script and style elements
    content = re.sub(r'(?is)<script[^>]*?>.*?</script>', '', content)
    content = re.sub(r'(?is)<style[^>]*?>.*?</style>', '', content)

    # 2. Split content by the markdown file/section delimiters
    pattern = r'(?m)^---\n([a-zA-Z0-9_\-\./]+)\n---\n'
    parts = re.split(pattern, content)
    
    sanitized_parts = []
    if parts[0].strip():
        if not is_boilerplate_block("", parts[0]):
            sanitized_parts.append(parts[0])
        
    removed_count = 0
    removed_bytes = 0
    
    for i in range(1, len(parts), 2):
        file_path = parts[i]
        file_content = parts[i+1]
        
        if is_boilerplate_block(file_path, file_content):
            removed_count += 1
            removed_bytes += len(file_content) + len(file_path) + 8
        else:
            sanitized_parts.append(f"---\n{file_path}\n---\n{file_content}")
            
    sanitized_content = "".join(sanitized_parts)
    return sanitized_content, removed_count, removed_bytes

def main():
    parser = argparse.ArgumentParser(description="Guarded Local Repair Utility for Memos RAG source content.")
    parser.add_argument("--dry-run", action="store_true", help="Run without mutating the database.")
    parser.add_argument("--tenant-id", type=int, default=6, help="Target tenant ID (default: 6).")
    parser.add_argument("--db-path", default="/home/chaschel/Documents/go/bchat/build/data/memos_dev.db", help="Path to database.")
    args = parser.parse_args()

    if not os.path.exists(args.db_path):
        print(f"Error: Database not found at {args.db_path}")
        sys.exit(1)

    print(f"--- RAG Source Content Local Repair Utility ---")
    print(f"Target Tenant ID: {args.tenant_id}")
    print(f"Database Path: {args.db_path}")
    print(f"Dry Run Mode: {args.dry_run}")
    print(f"-----------------------------------------------\n")

    conn = sqlite3.connect(args.db_path)
    cursor = conn.cursor()

    # Get the latest KB source file for the target tenant
    cursor.execute(
        "SELECT id, file_type, audience_type, content, version FROM agent_source_files WHERE tenant_id = ? AND file_type = 'kb' ORDER BY id DESC LIMIT 1",
        (args.tenant_id,)
    )
    row = cursor.fetchone()
    if not row:
        print(f"Error: No KB source file found for Tenant {args.tenant_id}")
        conn.close()
        sys.exit(1)

    source_id, file_type, audience_type, content, version = row
    orig_len = len(content)
    orig_hash = get_hash(content)

    print(f"Active KB Source File:")
    print(f"  Source ID: {source_id}")
    print(f"  File Type: {file_type}")
    print(f"  Audience: {audience_type}")
    print(f"  Version: {version}")
    print(f"  Original Length: {orig_len} bytes")
    print(f"  Original SHA-256: {orig_hash}\n")

    # Run sanitization
    sanitized_content, removed_count, removed_bytes = clean_source_content(content)
    san_len = len(sanitized_content)
    san_hash = get_hash(sanitized_content)

    if removed_count == 0:
        print("No script, style, or tracking boilerplate found. Content is already canonical. Idempotent exit.")
        conn.close()
        sys.exit(0)

    print(f"Sanitization Results:")
    print(f"  Removed Sections: {removed_count}")
    print(f"  Removed Size: {removed_bytes} bytes")
    print(f"  Sanitized Length: {san_len} bytes")
    print(f"  Sanitized SHA-256: {san_hash}\n")

    # Save backup locally in scratch/backups/
    backup_dir = "/home/chaschel/Documents/go/bchat/scratch/backups"
    os.makedirs(backup_dir, exist_ok=True)
    backup_path = os.path.join(backup_dir, f"kb_backup_tenant_{args.tenant_id}_v{version}_{orig_hash[:8]}.md")
    
    with open(backup_path, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"✓ Backup saved locally to: {backup_path}")

    # Write clean draft for review
    draft_path = os.path.join(backup_dir, f"kb_clean_draft_tenant_{args.tenant_id}_v{version}_{san_hash[:8]}.md")
    with open(draft_path, "w", encoding="utf-8") as f:
        f.write(sanitized_content)
    print(f"✓ Clean draft saved locally to: {draft_path}\n")

    if args.dry_run:
        print("Dry-run complete. No mutations were applied to the database.")
        conn.close()
        sys.exit(0)

    # Perform mutation inside an explicit transaction
    try:
        conn.execute("BEGIN TRANSACTION;")
        
        # 1. Update the content of the source file
        conn.execute(
            "UPDATE agent_source_files SET content = ?, content_hash = ?, version = version + 1 WHERE id = ?;",
            (sanitized_content, san_hash, source_id)
        )
        print("✓ KB source file content successfully sanitized and updated in database.")

        # 2. Reset the stalled reindex checkpoint back to failed/resumable or delete it
        # This allows a clean re-triggering of reindexing
        conn.execute(
            "DELETE FROM agent_reindex_checkpoints WHERE tenant_id = ? AND audience = ?;",
            (args.tenant_id, audience_type)
        )
        print("✓ Stalled reindex checkpoint deleted to enable clean indexing.")

        conn.commit()
        print("\nTransaction successfully committed! Local repair complete.")
    except Exception as e:
        conn.rollback()
        print(f"\nError: Transaction failed. Database rolled back successfully. Reason: {e}")
        conn.close()
        sys.exit(1)

    conn.close()

if __name__ == "__main__":
    main()
