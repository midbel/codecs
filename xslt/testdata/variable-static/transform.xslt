<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:variable name="lang" select="'en'" static="yes"/>
	<xsl:template use-when="$lang='en'" match="/">
		<item>foo</item>
	</xsl:template>
	<xsl:template use-when="$lang!='en'" match="/">
		<item>bar</item>
	</xsl:template>
</xsl:stylesheet>