import sqlite3

def print_start():
    db_path = '/home/chaschel/Documents/go/bchat/build/data/memos_dev.db'
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT content FROM agent_source_files WHERE tenant_id = 6 AND file_type = 'kb' ORDER BY id DESC LIMIT 1")
    row = cursor.fetchone()
    if not row:
        return
    
    content = row[0]
    print("--- FIRST 5000 CHARACTERS ---")
    print(content[:5000])
    
    print("\n--- POS 16000 TO 18000 (around 3hsp/index.md) ---")
    print(content[15500:18000])
    
    print("\n--- POS 390000 TO 393000 ---")
    print(content[390000:393000])

print_start()
