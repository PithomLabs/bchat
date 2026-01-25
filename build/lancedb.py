import lancedb                                                                
db = lancedb.connect("build/data/lancedb")                                    
table = db.open_table("kb_documents")                                         
print(table.to_pandas())  # Full data view  
