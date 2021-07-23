<?php
libxml_use_internal_errors(true);

$link = "https://www.cbf.com.br/futebol-brasileiro/competicoes/campeonato-brasileiro-serie-a/2021";
$url = 'html-' . date("m.d.y")  . '.html';
$time_array = getdata($url);

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

function volta($array)
{
    $array[1] = preg_replace('/\s+/', ' ', $array[1]);
    $array[1] = str_replace('0', '', $array[1]);
    $array[1] = str_replace('+1', '', $array[1]);
    $array[1] = str_replace('+2', '', $array[1]);
    $array[1] = str_replace('+3', '', $array[1]);
    $array[1] = str_replace('+4', '', $array[1]);
    $array[1] = str_replace('+5', '', $array[1]);
    $array[1] = str_replace('+6', '', $array[1]);
    $array[1] = str_replace('+7', '', $array[1]);
    $array[1] = str_replace('+8', '', $array[1]);
    $array[1] = str_replace('+9', '', $array[1]);
    $array[1] = str_replace('-1', '', $array[1]);
    $array[1] = str_replace('-2', '', $array[1]);
    $array[1] = str_replace('-3', '', $array[1]);
    $array[1] = str_replace('-4', '', $array[1]);
    $array[1] = str_replace('-5', '', $array[1]);
    $array[1] = str_replace('-6', '', $array[1]);
    $array[1] = str_replace('-7', '', $array[1]);
    $array[1] = str_replace('-8', '', $array[1]);
    $array[1] = str_replace('-9', '', $array[1]);


    $tabela = [
        'Equipe' => $array[1],
        'PTS' => $array[3],
        'J' => $array[5],
        'V' => $array[7],
        'E' => $array[9],
        'D' => $array[11],
        'GP' => $array[13],
        'GC' => $array[15],
        'SG' => $array[17],
        'CA' => $array[19],
        'CV' => $array[21],
        'POR' => $array[23]
        // $array[25],
        // $array[27]
    ];

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
        volta($time_array[35]),
        volta($time_array[37]),
        volta($time_array[39])
    );



libxml_use_internal_errors(false);
