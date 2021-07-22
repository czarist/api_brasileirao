<!DOCTYPE html>
<html lang="en">
<script src="https://code.jquery.com/jquery-3.6.0.js"></script>
<script>

</script>

<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Web scrap php</title>
</head>

<body>
    <?php
    libxml_use_internal_errors(true);

    function tdrows($elements)
    {
        $str = array();
        foreach ($elements as $element) {
            $str[] = trim($element->nodeValue);
        }


        return $str;
    }

    function getdata($contents)
    {
        $DOM = new DOMDocument;
        $DOM->loadHTMLFile($contents);
        $items = $DOM->getElementsByTagName('tr');
        $return = array();
        foreach ($items as $node) {
            $return[] = tdrows($node->childNodes);
        }

        return $return;
    }

    $url = "https://www.cbf.com.br/futebol-brasileiro/competicoes/campeonato-brasileiro-serie-a/2021";
    //$url = "teste.html";


    $json = json_encode(getdata($url), JSON_UNESCAPED_UNICODE);
    $search = array("\\", "\n", "\r", "\f", "\t", "\b", "\r\n");
    $replace = array("", "", "", "", "", "", "");
    $json = str_replace($search, $replace, $json);
    $time_array = getdata($url);
    //$array_map = array_map('trim', $array_api);


    function volta($array)
    {
        $array[1] = preg_replace('/\s+/', ' ', $array[1]);


        $tabela = array(
            $array[1],
            $array[3],
            $array[5],
            $array[7],
            $array[9],
            $array[11],
            $array[13],
            $array[15],
            $array[17],
            $array[19],
            $array[21],
            $array[23]
            // $array[25],
            // $array[27]
        );
        return $tabela;
    }

    $time_master =
        array(
            volta($time_array[1]),
            volta($time_array[3]),
            volta($time_array[5]),
            volta($time_array[7]),
            volta($time_array[9]),
            volta($time_array[11]),
            volta($time_array[13]),
            volta($time_array[15]),
            volta($time_array[17]),
            volta($time_array[19]),
            volta($time_array[21]),
            volta($time_array[23]),
            volta($time_array[25]),
            volta($time_array[27]),
            volta($time_array[29]),
            volta($time_array[31]),
            volta($time_array[33]),
            volta($time_array[35])
            // volta($time_array[37]),
            // volta($time_array[39])
        );

    echo "<pre>";
    print_r($time_master);
    echo "</pre>";



    ?>

    <script type="text/javascript">
        var json = <?php echo json_encode($time_master); ?>;
        console.log(json);
    </script>

</body>

</html>
<?php libxml_use_internal_errors(false);
?>