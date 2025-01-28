def started():
    try:
        open("/~~/api/Started")
    except: 
        pass # Should always fail due to non-existent file

def exited():
    try:
        open("/~~/api/Exited")
    except:
        pass # Should always fail due to non-existent file
