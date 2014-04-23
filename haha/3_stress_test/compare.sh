echo 'compare dumplogs...'
for file1 in $(find dumplog_*)
do
    for file2 in $(find dumplog_*)
    do
        if [ "$(diff $file1 $file2)" == '\n' ]
        then
            echo $file1 " is different from " $file2
        fi
    done
done
