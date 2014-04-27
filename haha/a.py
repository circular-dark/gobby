d = {}
with open('output') as f:
  for line in f:
    if line.strip():
      l=line.strip().split()
      print l[6],l[7],l[8]
      node = l[7].split(',')[0]
      seq = int(l[6].split(',')[0])

      if node in d:
	d[node].add(seq)
      else:
	d[node]=set()

for n in d:
  if len(d[n])< 50:
    rl = sorted(list(d[n]))
    print n,rl
