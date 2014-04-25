import sys

def main(name):
  d = {}
  with open(name) as f:
    for line in f:
      l = line.strip()
      if l:
	comm = l.split(':')[1]
	if comm in d:
	  d[comm] += 1
	  print comm, d[comm]
	else:
	  d[comm] = 1

if __name__ == "__main__":
  main(sys.argv[1])
