import sqlite3

class DBReader:
    def __init__(self, dbPath):
        self.con = sqlite3.connect(dbPath)

    def getRandomRecord(self):
        pass

    def getRecords(self):
        cur = self.con.cursor()
        return cur.execute("SELECT * FROM records").fetchall()

    def hasRecord(self, uuid):
        pass
