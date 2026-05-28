import sqlite3
import re

def test_sanitizer_refined():
    db_path = '/home/chaschel/Documents/go/bchat/build/data/memos_dev.db'
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT content FROM agent_source_files WHERE tenant_id = 6 AND file_type = 'kb' ORDER BY id DESC LIMIT 1")
    row = cursor.fetchone()
    if not row:
        print("No KB.MD found")
        return
    
    content = row[0]
    print(f"Original length: {len(content)}")
    
    pattern = r'(?m)^---\n([a-zA-Z0-9_\-\./]+)\n---\n'
    parts = re.split(pattern, content)
    
    sanitized_parts = []
    if parts[0].strip():
        sanitized_parts.append(parts[0])
        
    removed_count = 0
    removed_bytes = 0
    
    for i in range(1, len(parts), 2):
        file_path = parts[i]
        file_content = parts[i+1]
        
        is_boilerplate = False
        reason = ""
        
        # 1. Exact path matches for known boilerplate/script files
        if file_path.endswith(('.js', '.css')) or any(kw in file_path.lower() for kw in ['3hsp/index.md', 'googletagmanager', 'google_tag_manager']):
            is_boilerplate = True
            reason = f"File path indicates script/boilerplate: {file_path}"
            
        # 2. Body analysis: HTML style/script blocks
        if not is_boilerplate:
            if re.search(r'<(script|style).*?>', file_content, re.IGNORECASE):
                # Check if it contains actual script or if it's just documenting the tags.
                # If a block is completely enclosed inside a <script> tag:
                # E.g. <script>...</script> at the root of the file content
                # Let's remove script and style tags and their contents
                # But wait, if we remove just the tags/contents, or the whole file?
                # If the entire file is enclosed, we reject it. If it has some script, we can strip the script content.
                pass
                
        # 3. Detect minified JS/CSS boilerplate
        if not is_boilerplate:
            lines = file_content.split('\n')
            total_lines = len(lines)
            
            # Check for very long lines with typical JS signature
            for line_idx, line in enumerate(lines):
                line = line.strip()
                if len(line) > 500:
                    spaces = line.count(' ')
                    space_ratio = spaces / len(line)
                    
                    # Check JS signatures in the long line
                    js_signatures = ["(function(", "eval(", "window.", "document.", "var ", "const ", "let ", "function(", "dataLayer.push("]
                    has_js_sig = any(sig in line for sig in js_signatures)
                    
                    # Minified JS line signature: long line, extremely low space ratio, and contains typical JS patterns
                    if space_ratio < 0.05 and has_js_sig:
                        is_boilerplate = True
                        reason = f"Line {line_idx} has minified JS signature (len: {len(line)}, spaces: {spaces}, ratio: {space_ratio:.4f})"
                        break
                        
                    # Minified CSS signature: long line containing brackets and properties with very few spaces
                    if space_ratio < 0.05 and '{' in line and '}' in line and ';' in line:
                        is_boilerplate = True
                        reason = f"Line {line_idx} has minified CSS signature (len: {len(line)}, spaces: {spaces})"
                        break
                        
        if is_boilerplate:
            print(f"[STRIPPED] File: {file_path} ({len(file_content)} chars) - Reason: {reason}")
            removed_count += 1
            removed_bytes += len(file_content) + len(file_path) + 8
        else:
            sanitized_parts.append(f"---\n{file_path}\n---\n{file_content}")
            
    sanitized_content = "".join(sanitized_parts)
    print(f"\nRemoved {removed_count} files/sections totaling {removed_bytes} characters.")
    print(f"Sanitized length: {len(sanitized_content)} characters")
    
    # Check if we preserved the useful articles
    print("\n--- Preserved Headers Check ---")
    headers = [h for h in re.findall(r'^(#{1,6}\s+.*)$', sanitized_content, re.MULTILINE)]
    print(f"Found {len(headers)} headers in sanitized content:")
    for h in headers[:10]:
        print(f"  {h}")
    if len(headers) > 10:
        print(f"  ... and {len(headers) - 10} more")

test_sanitizer_refined()
